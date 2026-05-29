package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// BackupService runs encrypted Postgres dumps and uploads them to MinIO
// while keeping a local copy. The service also tracks every attempt in
// the db_backups table and applies GFS retention.
type BackupService struct {
	db     *database.DB
	cfg    *config.Config
	minio  *minio.Client
	logger *zap.Logger
}

// BackupRetention is the GFS rotation policy. Only successful backups
// count toward the cap; failed rows are pruned by age (90 days) so they
// stay around long enough to debug a regression.
type BackupRetention struct {
	Daily      int
	Weekly     int
	Monthly    int
	Adhoc      int
	FailedDays int
}

var defaultRetention = BackupRetention{
	Daily:      7,
	Weekly:     4,
	Monthly:    12,
	Adhoc:      5,
	FailedDays: 90,
}

// BackupRow mirrors a row of the db_backups table for handler+service use.
type BackupRow struct {
	ID          uuid.UUID  `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string     `json:"status"`
	Tier        string     `json:"tier"`
	SizeBytes   *int64     `json:"size_bytes,omitempty"`
	ObjectKey   *string    `json:"object_key,omitempty"`
	LocalPath   *string    `json:"local_path,omitempty"`
	TriggeredBy string     `json:"triggered_by"`
	AdminID     *string    `json:"admin_id,omitempty"`
	Error       *string    `json:"error,omitempty"`
}

// NewBackupService constructs the service. It builds its own minio.Client
// rather than reusing the upload-bucket Client so backup credentials and
// bucket lifecycle stay isolated.
func NewBackupService(db *database.DB, cfg *config.Config, logger *zap.Logger) (*BackupService, error) {
	mc, err := minio.New(cfg.Storage.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
		Region: cfg.Storage.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("backup minio client: %w", err)
	}
	return &BackupService{
		db:     db,
		cfg:    cfg,
		minio:  mc,
		logger: logger,
	}, nil
}

// ErrBackupDisabled is returned by Run when BACKUP_ENABLED is false or
// BACKUP_PASSPHRASE is empty. Callers should log+continue rather than
// crashing the process.
var ErrBackupDisabled = errors.New("backup disabled or passphrase missing")

// Run executes a full backup and records the attempt. triggeredBy is
// "cron" or "admin". adminID may be nil. Returns the new db_backups row
// id even when the backup itself fails so the UI can surface the error.
func (s *BackupService) Run(ctx context.Context, triggeredBy string, adminID *string) (uuid.UUID, error) {
	if !s.cfg.Backup.Enabled || s.cfg.Backup.Passphrase == "" {
		return uuid.Nil, ErrBackupDisabled
	}

	tier := classifyTier(time.Now().UTC(), triggeredBy)

	id := uuid.New()
	now := time.Now().UTC()
	if _, err := s.db.Pool.Exec(ctx,
		`INSERT INTO db_backups (id, started_at, status, tier, triggered_by, admin_id) VALUES ($1, $2, 'running', $3, $4, $5)`,
		id, now, tier, triggeredBy, adminID,
	); err != nil {
		return uuid.Nil, fmt.Errorf("insert db_backups: %w", err)
	}

	objectKey, localPath, size, runErr := s.execute(ctx, id, tier, now)

	finishedAt := time.Now().UTC()
	if runErr != nil {
		_, _ = s.db.Pool.Exec(ctx,
			`UPDATE db_backups SET completed_at=$1, status='failed', error=$2 WHERE id=$3`,
			finishedAt, runErr.Error(), id,
		)
		s.logger.Error("backup failed", zap.String("id", id.String()), zap.Error(runErr))
		return id, runErr
	}

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE db_backups SET completed_at=$1, status='success', size_bytes=$2, object_key=$3, local_path=$4 WHERE id=$5`,
		finishedAt, size, objectKey, localPath, id,
	); err != nil {
		return id, fmt.Errorf("update db_backups: %w", err)
	}
	s.logger.Info("backup completed",
		zap.String("id", id.String()),
		zap.String("tier", tier),
		zap.Int64("size_bytes", size),
		zap.String("object_key", objectKey),
	)
	return id, nil
}

// execute does the dump → encrypt → fan-out, returning artifact metadata.
// Local copy is written first; only after that succeeds do we upload to
// MinIO. If the upload fails the local copy is still kept (better than
// losing a known-good snapshot).
func (s *BackupService) execute(ctx context.Context, id uuid.UUID, tier string, ts time.Time) (objectKey, localPath string, size int64, err error) {
	if mkErr := os.MkdirAll(s.cfg.Backup.LocalDir, 0o750); mkErr != nil {
		return "", "", 0, fmt.Errorf("mkdir local dir: %w", mkErr)
	}

	stamp := ts.Format("20060102-150405")
	filename := fmt.Sprintf("hamsaya-%s-%s.dump.gpg", stamp, tier)
	localPath = filepath.Join(s.cfg.Backup.LocalDir, filename)

	passFile, err := writePassphraseFile(s.cfg.Backup.Passphrase)
	if err != nil {
		return "", "", 0, err
	}
	defer func() { _ = os.Remove(passFile) }()

	out, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", "", 0, fmt.Errorf("open local file: %w", err)
	}
	defer func() { _ = out.Close() }()

	port := s.cfg.Database.Port
	if port == "" {
		port = "5432"
	}

	dump := exec.CommandContext(ctx, "pg_dump",
		"-h", s.cfg.Database.Host,
		"-p", port,
		"-U", s.cfg.Database.User,
		"-d", s.cfg.Database.Name,
		"--format=custom",
		"--no-owner",
		"--no-privileges",
	)
	dump.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.Database.Password)
	dumpStderr, _ := dump.StderrPipe()
	dumpStdout, err := dump.StdoutPipe()
	if err != nil {
		return "", "", 0, fmt.Errorf("dump stdout pipe: %w", err)
	}

	gpg := exec.CommandContext(ctx, "gpg",
		"--symmetric",
		"--cipher-algo", "AES256",
		"--batch",
		"--yes",
		"--passphrase-file", passFile,
		"--no-tty",
	)
	gpg.Stdin = dumpStdout
	gpg.Stdout = out
	gpgStderr, _ := gpg.StderrPipe()

	if startErr := dump.Start(); startErr != nil {
		return "", "", 0, fmt.Errorf("start pg_dump: %w", startErr)
	}
	if startErr := gpg.Start(); startErr != nil {
		_ = dump.Process.Kill()
		return "", "", 0, fmt.Errorf("start gpg: %w", startErr)
	}

	dumpErrBuf, _ := io.ReadAll(dumpStderr)
	gpgErrBuf, _ := io.ReadAll(gpgStderr)

	if waitErr := dump.Wait(); waitErr != nil {
		_ = gpg.Process.Kill()
		_ = os.Remove(localPath)
		return "", "", 0, fmt.Errorf("pg_dump: %w (%s)", waitErr, string(dumpErrBuf))
	}
	if waitErr := gpg.Wait(); waitErr != nil {
		_ = os.Remove(localPath)
		return "", "", 0, fmt.Errorf("gpg: %w (%s)", waitErr, string(gpgErrBuf))
	}

	stat, err := out.Stat()
	if err != nil {
		return "", "", 0, fmt.Errorf("stat local file: %w", err)
	}
	size = stat.Size()
	if size == 0 {
		_ = os.Remove(localPath)
		return "", "", 0, fmt.Errorf("backup produced empty file")
	}

	if err := s.ensureBucket(ctx); err != nil {
		s.logger.Warn("backup bucket ensure failed; local copy retained",
			zap.String("local_path", localPath), zap.Error(err),
		)
		return "", localPath, size, nil
	}

	objectKey = fmt.Sprintf("%s/%s", tier, filename)
	uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if _, upErr := s.minio.FPutObject(uploadCtx, s.cfg.Backup.Bucket, objectKey, localPath, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
		UserMetadata: map[string]string{
			"tier":         tier,
			"db-backup-id": id.String(),
		},
	}); upErr != nil {
		s.logger.Warn("backup upload failed; local copy retained",
			zap.String("local_path", localPath), zap.Error(upErr),
		)
		return "", localPath, size, nil
	}

	return objectKey, localPath, size, nil
}

// ensureBucket creates the backup bucket on first run. Idempotent on MinIO.
//
// On Cloudflare R2 the typical "Object Read & Write" API token lacks the
// HeadBucket and CreateBucket permissions, so BucketExists + MakeBucket both
// return AccessDenied even when the bucket already exists. To avoid blocking
// uploads on a permission gap that operators can't fix without re-issuing
// the token, we skip the existence probe entirely against R2 endpoints and
// trust the operator to create the bucket via the Cloudflare dashboard. If
// the bucket really is missing the subsequent FPutObject call will surface
// the error and the caller already retains the local copy as a fallback.
func (s *BackupService) ensureBucket(ctx context.Context) error {
	if strings.Contains(s.cfg.Storage.Endpoint, "r2.cloudflarestorage.com") {
		return nil
	}
	exists, err := s.minio.BucketExists(ctx, s.cfg.Backup.Bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.minio.MakeBucket(ctx, s.cfg.Backup.Bucket, minio.MakeBucketOptions{Region: s.cfg.Storage.Region}); err != nil {
			return err
		}
		s.logger.Info("created backup bucket", zap.String("bucket", s.cfg.Backup.Bucket))
	}
	return nil
}

// List returns recent backup attempts, newest first.
func (s *BackupService) List(ctx context.Context, limit int) ([]BackupRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, started_at, completed_at, status, tier, size_bytes,
		       object_key, local_path, triggered_by, admin_id::text, error
		FROM db_backups
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BackupRow, 0, limit)
	for rows.Next() {
		var r BackupRow
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.CompletedAt, &r.Status, &r.Tier,
			&r.SizeBytes, &r.ObjectKey, &r.LocalPath, &r.TriggeredBy, &r.AdminID, &r.Error,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// OpenDownload returns a stream of the encrypted backup artifact and its
// size for streaming through the API server. We stream rather than
// presign because the MinIO endpoint baked into the client is the
// internal docker hostname and the signed Host header cannot be
// rewritten without invalidating the signature. Streaming through the
// API also keeps the artifact behind admin auth.
type DownloadStream struct {
	Reader   io.ReadCloser
	Size     int64
	Filename string
}

func (s *BackupService) OpenDownload(ctx context.Context, id string) (*DownloadStream, error) {
	var key *string
	var size *int64
	if err := s.db.Pool.QueryRow(ctx,
		`SELECT object_key, size_bytes FROM db_backups WHERE id=$1`, id,
	).Scan(&key, &size); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("backup not found")
		}
		return nil, err
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("backup has no object_key (upload failed or not finished)")
	}
	obj, err := s.minio.GetObject(ctx, s.cfg.Backup.Bucket, *key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, err
	}
	filename := filepath.Base(*key)
	out := &DownloadStream{
		Reader:   obj,
		Size:     stat.Size,
		Filename: filename,
	}
	if size != nil && *size > 0 && stat.Size != *size {
		s.logger.Warn("backup size drift", zap.Int64("recorded", *size), zap.Int64("object", stat.Size))
	}
	return out, nil
}

// Prune applies GFS retention. Returns the number of backups removed.
// Only `success` rows count toward the cap; `failed` rows are pruned by
// age.
func (s *BackupService) Prune(ctx context.Context) (int, error) {
	r := defaultRetention
	deleted := 0
	for tier, keep := range map[string]int{
		"daily":   r.Daily,
		"weekly":  r.Weekly,
		"monthly": r.Monthly,
		"adhoc":   r.Adhoc,
	} {
		rows, err := s.db.Pool.Query(ctx, `
			SELECT id, object_key, local_path FROM db_backups
			WHERE tier=$1 AND status='success'
			ORDER BY started_at DESC
			OFFSET $2
		`, tier, keep)
		if err != nil {
			return deleted, fmt.Errorf("prune query %s: %w", tier, err)
		}
		var stale []BackupRow
		for rows.Next() {
			var b BackupRow
			if err := rows.Scan(&b.ID, &b.ObjectKey, &b.LocalPath); err != nil {
				rows.Close()
				return deleted, err
			}
			stale = append(stale, b)
		}
		rows.Close()

		for _, b := range stale {
			if b.LocalPath != nil && *b.LocalPath != "" {
				_ = os.Remove(*b.LocalPath)
			}
			if b.ObjectKey != nil && *b.ObjectKey != "" {
				_ = s.minio.RemoveObject(ctx, s.cfg.Backup.Bucket, *b.ObjectKey, minio.RemoveObjectOptions{})
			}
			if _, err := s.db.Pool.Exec(ctx, `DELETE FROM db_backups WHERE id=$1`, b.ID); err == nil {
				deleted++
			}
		}
	}

	// Age-based prune for failed rows.
	if _, err := s.db.Pool.Exec(ctx,
		`DELETE FROM db_backups WHERE status='failed' AND started_at < NOW() - make_interval(days => $1)`,
		r.FailedDays,
	); err != nil {
		return deleted, fmt.Errorf("prune failed rows: %w", err)
	}

	if deleted > 0 {
		s.logger.Info("backup retention pruned", zap.Int("deleted", deleted))
	}
	return deleted, nil
}

// classifyTier picks a GFS tier from the timestamp. Adhoc trumps the
// date-based tiers so an admin-triggered backup never disturbs the daily
// rotation slot.
func classifyTier(ts time.Time, triggeredBy string) string {
	if triggeredBy == "admin" {
		return "adhoc"
	}
	if ts.Day() == 1 {
		return "monthly"
	}
	if ts.Weekday() == time.Sunday {
		return "weekly"
	}
	return "daily"
}

// writePassphraseFile writes the passphrase to a 0600 temp file. gpg's
// --passphrase-file argument is more robust than --passphrase-fd because
// it doesn't fight with stdin (which we use for the encrypted stream).
func writePassphraseFile(passphrase string) (string, error) {
	f, err := os.CreateTemp("", "hamsaya-bkp-*")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(passphrase); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

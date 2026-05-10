package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/pkg/database"
)

// MediaModerationService runs the admin-facing /media-moderation queue:
// list pending media, approve (no-op metadata change), reject (soft
// delete the underlying attachment so it stops being served).
type MediaModerationService struct {
	db     *database.DB
	logger *zap.Logger
}

func NewMediaModerationService(db *database.DB, logger *zap.Logger) *MediaModerationService {
	return &MediaModerationService{db: db, logger: logger}
}

// MediaModerationItem mirrors a queue row joined with the attachment
// payload + author info so the UI can render previews + context.
type MediaModerationItem struct {
	AttachmentID  string     `json:"attachment_id"`
	PostID        string     `json:"post_id"`
	UserID        string     `json:"user_id"`
	UserEmail     string     `json:"user_email"`
	PostTitle     string     `json:"post_title"`
	PostType      string     `json:"post_type"`
	MediaURL      string     `json:"media_url"`
	MediaName     string     `json:"media_name"`
	MimeType      string     `json:"mime_type"`
	EnqueuedAt    time.Time  `json:"enqueued_at"`
	Status        string     `json:"status"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy    *string    `json:"reviewed_by,omitempty"`
	ReviewerEmail *string    `json:"reviewer_email,omitempty"`
	ReviewNotes   *string    `json:"review_notes,omitempty"`
}

// List returns up to `limit` items, optionally filtered by status.
// Newest pending first by default; reviewed items grouped at the bottom.
func (s *MediaModerationService) List(ctx context.Context, status string, limit int) ([]MediaModerationItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT q.attachment_id::text, q.post_id::text,
		       p.user_id::text, COALESCE(u.email,''),
		       COALESCE(p.title, ''), COALESCE(p.type, ''),
		       a.photo->>'url',  a.photo->>'name',  a.photo->>'mime_type',
		       q.enqueued_at, q.status,
		       q.reviewed_at, q.reviewed_by::text, COALESCE(rv.email,''),
		       q.review_notes
		FROM media_moderation_queue q
		JOIN attachments a ON a.id = q.attachment_id
		LEFT JOIN posts p   ON p.id = q.post_id
		LEFT JOIN users u   ON u.id = p.user_id
		LEFT JOIN users rv  ON rv.id = q.reviewed_by
	`
	args := []interface{}{}
	if status != "" {
		q += " WHERE q.status = $1"
		args = append(args, status)
	}
	q += " ORDER BY (q.status = 'pending') DESC, q.enqueued_at DESC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := s.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MediaModerationItem, 0, limit)
	for rows.Next() {
		var it MediaModerationItem
		var reviewerEmail string
		if err := rows.Scan(
			&it.AttachmentID, &it.PostID,
			&it.UserID, &it.UserEmail,
			&it.PostTitle, &it.PostType,
			&it.MediaURL, &it.MediaName, &it.MimeType,
			&it.EnqueuedAt, &it.Status,
			&it.ReviewedAt, &it.ReviewedBy, &reviewerEmail,
			&it.ReviewNotes,
		); err != nil {
			return nil, err
		}
		if reviewerEmail != "" {
			it.ReviewerEmail = &reviewerEmail
		}
		out = append(out, it)
	}
	return out, nil
}

// Counts returns the size of each status bucket. Used for the queue
// dashboard cards.
func (s *MediaModerationService) Counts(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Pool.Query(ctx, `SELECT status, COUNT(*) FROM media_moderation_queue GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{"pending": 0, "approved": 0, "rejected": 0}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, nil
}

// Approve marks an item approved. No-op on the underlying attachment;
// the post + media keep serving as before.
func (s *MediaModerationService) Approve(ctx context.Context, attachmentID, adminID, notes string) error {
	tag, err := s.db.Pool.Exec(ctx, `
		UPDATE media_moderation_queue
		SET status='approved', reviewed_at=NOW(), reviewed_by=$1, review_notes=NULLIF($2,'')
		WHERE attachment_id=$3 AND status='pending'
	`, adminID, notes, attachmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no pending row for attachment %s", attachmentID)
	}
	return nil
}

// Reject marks the item rejected AND soft-deletes the attachment so it
// stops appearing on the post. The post itself stays alive — admins can
// take down the whole post via the existing report/moderation flow if
// the rejection signals broader abuse.
func (s *MediaModerationService) Reject(ctx context.Context, attachmentID, adminID, notes string) error {
	if notes == "" {
		return fmt.Errorf("rejection requires notes")
	}
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the queue row to avoid double-rejects from racing admins.
	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM media_moderation_queue WHERE attachment_id=$1 FOR UPDATE`,
		attachmentID,
	).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("queue row not found")
		}
		return err
	}
	if status != "pending" {
		return fmt.Errorf("already %s; cannot reject", status)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE media_moderation_queue
		SET status='rejected', reviewed_at=NOW(), reviewed_by=$1, review_notes=$2
		WHERE attachment_id=$3
	`, adminID, notes, attachmentID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE attachments SET deleted_at = NOW() WHERE id = $1`,
		attachmentID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

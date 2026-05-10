package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// DeletionRequestService manages GDPR-style account deletion requests.
// The flow: admin (or self-service in a future iteration) creates a
// pending request → another admin reviews and approves/rejects → on
// approval, the actual user delete is run and status flips to completed.
// Splitting request from execution gives a paper trail for compliance.
type DeletionRequestService struct {
	db           *database.DB
	adminService *AdminService
	logger       *zap.Logger
}

func NewDeletionRequestService(db *database.DB, adminService *AdminService, logger *zap.Logger) *DeletionRequestService {
	return &DeletionRequestService{db: db, adminService: adminService, logger: logger}
}

type DeletionRequestRow struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	UserEmail     string     `json:"user_email"`
	RequestedAt   time.Time  `json:"requested_at"`
	Reason        *string    `json:"reason,omitempty"`
	UserIP        *string    `json:"user_ip,omitempty"`
	Status        string     `json:"status"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy    *string    `json:"reviewed_by,omitempty"`
	ReviewerEmail *string    `json:"reviewer_email,omitempty"`
	ReviewNotes   *string    `json:"review_notes,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// Create enqueues a new deletion request on behalf of a user. Used both
// for third-party DSAR requests an admin handles in support and for the
// admin-initiated path. The user_ip column is populated from the
// requesting client where appropriate.
func (s *DeletionRequestService) Create(ctx context.Context, userID, reason, ip string) (uuid.UUID, error) {
	id := uuid.New()
	if _, err := s.db.Pool.Exec(ctx, `
		INSERT INTO account_deletion_requests (id, user_id, reason, user_ip)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))
	`, id, userID, reason, ip); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// List returns recent requests with optional status filter.
func (s *DeletionRequestService) List(ctx context.Context, status string, limit int) ([]DeletionRequestRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT r.id, r.user_id, COALESCE(u.email,''), r.requested_at, r.reason,
		       r.user_ip, r.status, r.reviewed_at, r.reviewed_by::text,
		       rv.email, r.review_notes, r.completed_at
		FROM account_deletion_requests r
		LEFT JOIN users u  ON u.id = r.user_id
		LEFT JOIN users rv ON rv.id = r.reviewed_by
	`
	args := []interface{}{}
	if status != "" {
		q += ` WHERE r.status = $1`
		args = append(args, status)
	}
	q += ` ORDER BY r.requested_at DESC LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := s.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeletionRequestRow, 0, limit)
	for rows.Next() {
		var r DeletionRequestRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserEmail, &r.RequestedAt, &r.Reason,
			&r.UserIP, &r.Status, &r.ReviewedAt, &r.ReviewedBy, &r.ReviewerEmail,
			&r.ReviewNotes, &r.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Approve marks a request approved AND executes the user delete. The
// review_notes field captures any rationale.
func (s *DeletionRequestService) Approve(ctx context.Context, id, adminID, notes string) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	var status string
	if err := tx.QueryRow(ctx,
		`SELECT user_id, status FROM account_deletion_requests WHERE id=$1 FOR UPDATE`,
		id,
	).Scan(&userID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("request not found")
		}
		return err
	}
	if status != "pending" {
		return fmt.Errorf("request is %s, not pending", status)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE account_deletion_requests
		SET status='approved', reviewed_at=NOW(), reviewed_by=$1, review_notes=NULLIF($2,'')
		WHERE id=$3
	`, adminID, notes, id); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Run actual delete via AdminService outside the tx (it has its own
	// internal transaction). On failure, the request stays approved but
	// not completed — admin can retry.
	if err := s.adminService.DeleteUser(ctx, userID.String(), adminID); err != nil {
		s.logger.Error("approve→delete failed", zap.String("request_id", id), zap.Error(err))
		return err
	}

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE account_deletion_requests SET status='completed', completed_at=NOW() WHERE id=$1`,
		id,
	); err != nil {
		s.logger.Warn("status→completed failed", zap.String("request_id", id), zap.Error(err))
	}
	return nil
}

// Reject closes a request without deleting the user. notes is required
// (admins must explain rejections — kept on file for compliance).
func (s *DeletionRequestService) Reject(ctx context.Context, id, adminID, notes string) error {
	if notes == "" {
		return fmt.Errorf("rejection requires notes")
	}
	tag, err := s.db.Pool.Exec(ctx, `
		UPDATE account_deletion_requests
		SET status='rejected', reviewed_at=NOW(), reviewed_by=$1, review_notes=$2
		WHERE id=$3 AND status='pending'
	`, adminID, notes, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no pending request with id %s", id)
	}
	return nil
}

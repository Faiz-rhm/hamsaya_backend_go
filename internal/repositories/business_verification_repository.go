package repositories

import (
	"context"
	"errors"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// ErrVerificationNotFound is returned when a verification request doesn't exist.
var ErrVerificationNotFound = errors.New("verification request not found")

// ErrVerificationPending is returned when the business already has an open request.
var ErrVerificationPending = errors.New("verification request already pending")

// BusinessVerificationRepository persists owner verification requests and
// admin review decisions.
type BusinessVerificationRepository interface {
	Create(ctx context.Context, req *models.BusinessVerificationRequest) error
	// GetLatestByBusiness returns the most recent request for a business, or
	// ErrVerificationNotFound.
	GetLatestByBusiness(ctx context.Context, businessID string) (*models.BusinessVerificationRequest, error)
	GetByID(ctx context.Context, id string) (*models.BusinessVerificationRequest, error)
	// List returns admin queue rows (optionally filtered by status) plus total.
	List(ctx context.Context, status *string, limit, offset int) ([]*models.BusinessVerificationListItem, int, error)
	// Review sets APPROVED/REJECTED + reviewer; only PENDING rows transition.
	Review(ctx context.Context, id, reviewerID, status string, reason *string) error
	// SetBusinessVerified flips the tick on the business itself.
	SetBusinessVerified(ctx context.Context, businessID string, verified bool) error
}

type businessVerificationRepository struct {
	db *database.DB
}

// NewBusinessVerificationRepository creates the repository.
func NewBusinessVerificationRepository(db *database.DB) BusinessVerificationRepository {
	return &businessVerificationRepository{db: db}
}

func (r *businessVerificationRepository) Create(ctx context.Context, req *models.BusinessVerificationRequest) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO business_verification_requests
			(id, business_id, user_id, license_no, note, documents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', NOW(), NOW())
	`, req.ID, req.BusinessID, req.UserID, req.LicenseNo, req.Note, req.Documents)
	if err != nil && isUniqueViolation(err) {
		return ErrVerificationPending
	}
	return err
}

// isUniqueViolation reports whether err is a Postgres 23505 unique violation.
func isUniqueViolation(err error) bool {
	type coder interface{ SQLState() string }
	var c coder
	if errors.As(err, &c) {
		return c.SQLState() == "23505"
	}
	return false
}

const verificationColumns = `
	id, business_id, user_id, license_no, note, documents, status,
	rejection_reason, reviewed_by, reviewed_at, created_at, updated_at`

func scanVerification(row pgx.Row) (*models.BusinessVerificationRequest, error) {
	v := &models.BusinessVerificationRequest{}
	err := row.Scan(
		&v.ID, &v.BusinessID, &v.UserID, &v.LicenseNo, &v.Note, &v.Documents,
		&v.Status, &v.RejectionReason, &v.ReviewedBy, &v.ReviewedAt,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrVerificationNotFound
	}
	return v, err
}

func (r *businessVerificationRepository) GetLatestByBusiness(ctx context.Context, businessID string) (*models.BusinessVerificationRequest, error) {
	return scanVerification(r.db.Pool.QueryRow(ctx, `
		SELECT`+verificationColumns+`
		FROM business_verification_requests
		WHERE business_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, businessID))
}

func (r *businessVerificationRepository) GetByID(ctx context.Context, id string) (*models.BusinessVerificationRequest, error) {
	return scanVerification(r.db.Pool.QueryRow(ctx, `
		SELECT`+verificationColumns+`
		FROM business_verification_requests
		WHERE id = $1
	`, id))
}

func (r *businessVerificationRepository) List(ctx context.Context, status *string, limit, offset int) ([]*models.BusinessVerificationListItem, int, error) {
	var total int
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM business_verification_requests v
		WHERE ($1::text IS NULL OR v.status = $1)
	`, status).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			v.id, v.business_id, v.user_id, v.license_no, v.note, v.documents,
			v.status, v.rejection_reason, v.reviewed_by, v.reviewed_at,
			v.created_at, v.updated_at,
			b.name, b.avatar, u.email
		FROM business_verification_requests v
		JOIN business_profiles b ON b.id = v.business_id
		JOIN users u ON u.id = v.user_id
		WHERE ($1::text IS NULL OR v.status = $1)
		ORDER BY v.created_at ASC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*models.BusinessVerificationListItem, 0, limit)
	for rows.Next() {
		item := &models.BusinessVerificationListItem{}
		if err := rows.Scan(
			&item.ID, &item.BusinessID, &item.UserID, &item.LicenseNo, &item.Note,
			&item.Documents, &item.Status, &item.RejectionReason, &item.ReviewedBy,
			&item.ReviewedAt, &item.CreatedAt, &item.UpdatedAt,
			&item.BusinessName, &item.BusinessAvatar, &item.OwnerEmail,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *businessVerificationRepository) Review(ctx context.Context, id, reviewerID, status string, reason *string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE business_verification_requests
		SET status = $2, rejection_reason = $3, reviewed_by = $4,
		    reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING'
	`, id, status, reason, reviewerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVerificationNotFound
	}
	return nil
}

func (r *businessVerificationRepository) SetBusinessVerified(ctx context.Context, businessID string, verified bool) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE business_profiles
		SET is_verified = $2,
		    verified_at = CASE WHEN $2 THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, businessID, verified)
	return err
}

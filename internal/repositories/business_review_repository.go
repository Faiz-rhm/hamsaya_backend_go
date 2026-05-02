package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// BusinessReviewRepository handles persistence for business reviews.
//
// Aggregates (avg_rating + review_count on business_profiles) are maintained
// by a database trigger, so callers here only deal with the row state of the
// reviews themselves.
type BusinessReviewRepository interface {
	// Upsert creates or replaces the caller's review for a business profile.
	// One review per (business_profile_id, user_id) is enforced by a unique
	// constraint, so this is naturally idempotent for the calling user.
	Upsert(ctx context.Context, review *models.BusinessReview) error

	// Update edits an existing review by id, scoped to the owning user.
	Update(ctx context.Context, reviewID, userID string, rating *int, comment *string) (*models.BusinessReview, error)

	// Delete removes a review. If allowAdmin is true the row is removed
	// regardless of authorship (used by admin endpoints).
	Delete(ctx context.Context, reviewID, userID string, allowAdmin bool) error

	// SetHidden toggles moderation visibility (admin-only flow).
	SetHidden(ctx context.Context, reviewID string, hidden bool) error

	// GetByID returns a single review.
	GetByID(ctx context.Context, reviewID string) (*models.BusinessReview, error)

	// GetByBusinessAndUser returns the calling user's existing review (or nil).
	GetByBusinessAndUser(ctx context.Context, businessID, userID string) (*models.BusinessReview, error)

	// ListByBusiness returns paginated reviews enriched with author info.
	// includeHidden=false hides moderated rows for non-admins.
	ListByBusiness(ctx context.Context, businessID string, includeHidden bool, limit, offset int) ([]*models.BusinessReviewWithAuthor, int, error)

	// GetStats returns aggregates for the summary card.
	GetStats(ctx context.Context, businessID string) (*models.BusinessReviewStats, error)
}

type businessReviewRepository struct {
	db *database.DB
}

// NewBusinessReviewRepository wires a new review repository.
func NewBusinessReviewRepository(db *database.DB) BusinessReviewRepository {
	return &businessReviewRepository{db: db}
}

// ErrReviewNotFound is returned when a review id doesn't exist or doesn't
// belong to the calling user (in non-admin contexts).
var ErrReviewNotFound = errors.New("review not found")

func (r *businessReviewRepository) Upsert(ctx context.Context, review *models.BusinessReview) error {
	const q = `
		INSERT INTO business_reviews (id, business_profile_id, user_id, rating, comment, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (business_profile_id, user_id)
		DO UPDATE SET rating = EXCLUDED.rating,
		              comment = EXCLUDED.comment,
		              updated_at = NOW()
		RETURNING id, created_at, updated_at, is_hidden
	`
	return r.db.Pool.QueryRow(ctx, q,
		review.ID,
		review.BusinessProfileID,
		review.UserID,
		review.Rating,
		review.Comment,
	).Scan(&review.ID, &review.CreatedAt, &review.UpdatedAt, &review.IsHidden)
}

func (r *businessReviewRepository) Update(ctx context.Context, reviewID, userID string, rating *int, comment *string) (*models.BusinessReview, error) {
	const q = `
		UPDATE business_reviews
		SET rating  = COALESCE($1, rating),
		    comment = COALESCE($2, comment),
		    updated_at = NOW()
		WHERE id = $3 AND user_id = $4
		RETURNING id, business_profile_id, user_id, rating, comment, is_hidden, created_at, updated_at
	`
	row := r.db.Pool.QueryRow(ctx, q, rating, comment, reviewID, userID)
	out := &models.BusinessReview{}
	err := row.Scan(
		&out.ID, &out.BusinessProfileID, &out.UserID, &out.Rating,
		&out.Comment, &out.IsHidden, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update review: %w", err)
	}
	return out, nil
}

func (r *businessReviewRepository) Delete(ctx context.Context, reviewID, userID string, allowAdmin bool) error {
	var (
		tag interface{ RowsAffected() int64 }
		err error
	)
	if allowAdmin {
		t, e := r.db.Pool.Exec(ctx, `DELETE FROM business_reviews WHERE id = $1`, reviewID)
		tag, err = t, e
	} else {
		t, e := r.db.Pool.Exec(ctx, `DELETE FROM business_reviews WHERE id = $1 AND user_id = $2`, reviewID, userID)
		tag, err = t, e
	}
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

func (r *businessReviewRepository) SetHidden(ctx context.Context, reviewID string, hidden bool) error {
	tag, err := r.db.Pool.Exec(ctx, `UPDATE business_reviews SET is_hidden = $1, updated_at = NOW() WHERE id = $2`, hidden, reviewID)
	if err != nil {
		return fmt.Errorf("set hidden: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

func (r *businessReviewRepository) GetByID(ctx context.Context, reviewID string) (*models.BusinessReview, error) {
	const q = `
		SELECT id, business_profile_id, user_id, rating, comment, is_hidden, created_at, updated_at
		FROM business_reviews
		WHERE id = $1
	`
	out := &models.BusinessReview{}
	err := r.db.Pool.QueryRow(ctx, q, reviewID).Scan(
		&out.ID, &out.BusinessProfileID, &out.UserID, &out.Rating,
		&out.Comment, &out.IsHidden, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get review: %w", err)
	}
	return out, nil
}

func (r *businessReviewRepository) GetByBusinessAndUser(ctx context.Context, businessID, userID string) (*models.BusinessReview, error) {
	const q = `
		SELECT id, business_profile_id, user_id, rating, comment, is_hidden, created_at, updated_at
		FROM business_reviews
		WHERE business_profile_id = $1 AND user_id = $2
	`
	out := &models.BusinessReview{}
	err := r.db.Pool.QueryRow(ctx, q, businessID, userID).Scan(
		&out.ID, &out.BusinessProfileID, &out.UserID, &out.Rating,
		&out.Comment, &out.IsHidden, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get review by user: %w", err)
	}
	return out, nil
}

func (r *businessReviewRepository) ListByBusiness(ctx context.Context, businessID string, includeHidden bool, limit, offset int) ([]*models.BusinessReviewWithAuthor, int, error) {
	hideClause := "AND r.is_hidden = FALSE"
	if includeHidden {
		hideClause = ""
	}

	listQ := fmt.Sprintf(`
		SELECT
			r.id, r.business_profile_id, r.user_id, r.rating, r.comment,
			r.is_hidden, r.created_at, r.updated_at,
			p.first_name, p.last_name, p.avatar, p.avatar_color
		FROM business_reviews r
		LEFT JOIN profiles p ON p.id = r.user_id
		WHERE r.business_profile_id = $1 %s
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, hideClause)

	rows, err := r.db.Pool.Query(ctx, listQ, businessID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	out := make([]*models.BusinessReviewWithAuthor, 0)
	for rows.Next() {
		w := &models.BusinessReviewWithAuthor{}
		if err := rows.Scan(
			&w.ID, &w.BusinessProfileID, &w.UserID, &w.Rating, &w.Comment,
			&w.IsHidden, &w.CreatedAt, &w.UpdatedAt,
			&w.AuthorFirstName, &w.AuthorLastName, &w.AuthorAvatar, &w.AuthorAvatarHex,
		); err != nil {
			return nil, 0, fmt.Errorf("scan review: %w", err)
		}
		out = append(out, w)
	}

	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM business_reviews r WHERE r.business_profile_id = $1 %s`, hideClause)
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQ, businessID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reviews: %w", err)
	}

	return out, total, nil
}

func (r *businessReviewRepository) GetStats(ctx context.Context, businessID string) (*models.BusinessReviewStats, error) {
	// Distribution: one COUNT per star bucket. Cheap on a small per-business table.
	const q = `
		SELECT
			COALESCE(AVG(rating)::numeric, 0)::float8 AS avg_rating,
			COUNT(*)                                  AS total,
			COUNT(*) FILTER (WHERE rating = 1)        AS r1,
			COUNT(*) FILTER (WHERE rating = 2)        AS r2,
			COUNT(*) FILTER (WHERE rating = 3)        AS r3,
			COUNT(*) FILTER (WHERE rating = 4)        AS r4,
			COUNT(*) FILTER (WHERE rating = 5)        AS r5
		FROM business_reviews
		WHERE business_profile_id = $1 AND is_hidden = FALSE
	`
	out := &models.BusinessReviewStats{BusinessProfileID: businessID}
	if err := r.db.Pool.QueryRow(ctx, q, businessID).Scan(
		&out.AvgRating, &out.ReviewCount,
		&out.Distribution[0], &out.Distribution[1], &out.Distribution[2],
		&out.Distribution[3], &out.Distribution[4],
	); err != nil {
		return nil, fmt.Errorf("review stats: %w", err)
	}
	return out, nil
}

package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// MonetizationRepository covers ads / credits / boosts.
//
// Combined into one repo because the domain is small and the three tables
// share a common admin-panel call site. If any sub-area grows beyond a
// dozen queries it should be split out.
type MonetizationRepository interface {
	// Ads
	ListAds(ctx context.Context, status string, page, limit int) ([]*models.Ad, int, error)
	ListActiveAds(ctx context.Context, limit int, province, language string) ([]*models.Ad, error)
	GetAd(ctx context.Context, id string) (*models.Ad, error)
	CreateAd(ctx context.Context, advertiserID, title, body, imageURL, targetURL, phoneNumber, whatsappNumber, status string, startAt, endAt *time.Time, weight int, dailyImpressionCap *int, targetProvinces, targetLanguages []string) (*models.Ad, error)
	UpdateAdStatus(ctx context.Context, id, status, reviewedBy string, req *models.AdReviewRequest) (*models.Ad, error)
	DeleteAd(ctx context.Context, id string) error
	IncrementAdImpression(ctx context.Context, id string) error
	IncrementAdClick(ctx context.Context, id string) error

	// Credits
	ListBalances(ctx context.Context, search string, page, limit int) ([]*models.CreditBalance, int, error)
	GetBalance(ctx context.Context, userID string) (*models.CreditBalance, error)
	ListUserTransactions(ctx context.Context, userID string, limit int) ([]*models.CreditTransaction, error)
	AdjustCredits(ctx context.Context, userID string, req *models.AdjustCreditsRequest, adminID string) (*models.CreditBalance, error)

	// Boosts
	ListBoosts(ctx context.Context, status string, page, limit int) ([]*models.Boost, int, error)
	GetBoost(ctx context.Context, id string) (*models.Boost, error)
	CancelBoost(ctx context.Context, id, adminID, reason string) (*models.Boost, error)
}

type monetizationRepository struct {
	db *database.DB
}

func NewMonetizationRepository(db *database.DB) MonetizationRepository {
	return &monetizationRepository{db: db}
}

// ─── Ads ─────────────────────────────────────────────────────────────────────

func (r *monetizationRepository) ListAds(ctx context.Context, status string, page, limit int) ([]*models.Ad, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	args := []any{}
	where := ""
	if status != "" {
		where = "WHERE a.status = $1"
		args = append(args, status)
	}

	// Count first.
	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM ads a "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ads count: %w", err)
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT a.id, a.advertiser_id,
		       COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), ''),
		       a.title, a.body, a.image_url, a.target_url,
		       a.phone_number, a.whatsapp_number,
		       a.weight, a.daily_impression_cap,
		       COALESCE(a.target_provinces, '{}'::text[]),
		       COALESCE(a.target_languages, '{}'::text[]),
		       a.status,
		       a.start_at, a.end_at, a.impressions, a.clicks,
		       a.reviewed_by::text, a.reviewed_at, a.review_note,
		       a.created_at, a.updated_at
		FROM ads a
		LEFT JOIN users u          ON u.id = a.advertiser_id
		LEFT JOIN profiles up ON up.id = a.advertiser_id
		%s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ads list: %w", err)
	}
	defer rows.Close()

	var out []*models.Ad
	for rows.Next() {
		ad := &models.Ad{}
		if err := rows.Scan(
			&ad.ID, &ad.AdvertiserID, &ad.AdvertiserEmail, &ad.AdvertiserName,
			&ad.Title, &ad.Body, &ad.ImageURL, &ad.TargetURL,
			&ad.PhoneNumber, &ad.WhatsAppNumber,
			&ad.Weight, &ad.DailyImpressionCap,
			&ad.TargetProvinces, &ad.TargetLanguages,
			&ad.Status,
			&ad.StartAt, &ad.EndAt, &ad.Impressions, &ad.Clicks,
			&ad.ReviewedBy, &ad.ReviewedAt, &ad.ReviewNote,
			&ad.CreatedAt, &ad.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("ads scan: %w", err)
		}
		out = append(out, ad)
	}
	return out, total, rows.Err()
}

// ListActiveAds returns ads that are currently servable: status=ACTIVE,
// optional start_at <= now, optional end_at >= now (NULL end_at = no expiry).
// Used by the public /v1/ads/active endpoint that mobile feeds consume.
//
// Targeting + frequency cap (when provided):
//   • [province] — empty target_provinces → match all; otherwise must contain it.
//   • [language] — empty target_languages → match all; otherwise must contain it.
//   • daily_impression_cap — when set, only return ads whose impressions
//     since today's UTC midnight stay below the cap. We approximate by using
//     the running counter since the last reset; a separate Redis-backed
//     per-day counter is the next iteration. For now: ads.impressions is the
//     lifetime counter so cap_skip is best-effort only when start_at is today
//     (covers the common "campaign launches today, cap impressions today").
//   • weight — selection bias. ORDER BY weight*RANDOM() DESC.
func (r *monetizationRepository) ListActiveAds(ctx context.Context, limit int, province, language string) ([]*models.Ad, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	const q = `
		SELECT a.id, a.advertiser_id,
		       COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), ''),
		       a.title, a.body, a.image_url, a.target_url,
		       a.phone_number, a.whatsapp_number,
		       a.weight, a.daily_impression_cap,
		       COALESCE(a.target_provinces, '{}'::text[]),
		       COALESCE(a.target_languages, '{}'::text[]),
		       a.status,
		       a.start_at, a.end_at, a.impressions, a.clicks,
		       a.reviewed_by::text, a.reviewed_at, a.review_note,
		       a.created_at, a.updated_at
		FROM ads a
		LEFT JOIN users u    ON u.id = a.advertiser_id
		LEFT JOIN profiles up ON up.id = a.advertiser_id
		WHERE a.status = 'ACTIVE'
		  AND (a.start_at IS NULL OR a.start_at <= NOW())
		  AND (a.end_at   IS NULL OR a.end_at   >  NOW())
		  AND (
		        a.daily_impression_cap IS NULL
		     OR a.start_at IS NULL
		     OR DATE(a.start_at AT TIME ZONE 'UTC') <> DATE(NOW() AT TIME ZONE 'UTC')
		     OR a.impressions < a.daily_impression_cap
		      )
		  AND (cardinality(a.target_provinces) = 0 OR $2 = ANY(a.target_provinces))
		  AND (cardinality(a.target_languages) = 0 OR $3 = ANY(a.target_languages))
		ORDER BY GREATEST(a.weight, 1) * RANDOM() DESC
		LIMIT $1
	`
	rows, err := r.db.Pool.Query(ctx, q, limit, province, language)
	if err != nil {
		return nil, fmt.Errorf("active ads list: %w", err)
	}
	defer rows.Close()

	var out []*models.Ad
	for rows.Next() {
		ad := &models.Ad{}
		if err := rows.Scan(
			&ad.ID, &ad.AdvertiserID, &ad.AdvertiserEmail, &ad.AdvertiserName,
			&ad.Title, &ad.Body, &ad.ImageURL, &ad.TargetURL,
			&ad.PhoneNumber, &ad.WhatsAppNumber,
			&ad.Weight, &ad.DailyImpressionCap,
			&ad.TargetProvinces, &ad.TargetLanguages,
			&ad.Status,
			&ad.StartAt, &ad.EndAt, &ad.Impressions, &ad.Clicks,
			&ad.ReviewedBy, &ad.ReviewedAt, &ad.ReviewNote,
			&ad.CreatedAt, &ad.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("active ads scan: %w", err)
		}
		out = append(out, ad)
	}
	return out, rows.Err()
}

// IncrementAdImpression bumps the impressions counter. Best-effort — if the
// row no longer exists it's a no-op.
func (r *monetizationRepository) IncrementAdImpression(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE ads SET impressions = impressions + 1 WHERE id = $1`, id)
	return err
}

// IncrementAdClick bumps the clicks counter.
func (r *monetizationRepository) IncrementAdClick(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE ads SET clicks = clicks + 1 WHERE id = $1`, id)
	return err
}

func (r *monetizationRepository) GetAd(ctx context.Context, id string) (*models.Ad, error) {
	const q = `
		SELECT a.id, a.advertiser_id,
		       COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), ''),
		       a.title, a.body, a.image_url, a.target_url,
		       a.phone_number, a.whatsapp_number,
		       a.weight, a.daily_impression_cap,
		       COALESCE(a.target_provinces, '{}'::text[]),
		       COALESCE(a.target_languages, '{}'::text[]),
		       a.status,
		       a.start_at, a.end_at, a.impressions, a.clicks,
		       a.reviewed_by::text, a.reviewed_at, a.review_note,
		       a.created_at, a.updated_at
		FROM ads a
		LEFT JOIN users u          ON u.id = a.advertiser_id
		LEFT JOIN profiles up ON up.id = a.advertiser_id
		WHERE a.id = $1
	`
	ad := &models.Ad{}
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&ad.ID, &ad.AdvertiserID, &ad.AdvertiserEmail, &ad.AdvertiserName,
		&ad.Title, &ad.Body, &ad.ImageURL, &ad.TargetURL,
		&ad.PhoneNumber, &ad.WhatsAppNumber,
		&ad.Weight, &ad.DailyImpressionCap,
		&ad.TargetProvinces, &ad.TargetLanguages,
		&ad.Status,
		&ad.StartAt, &ad.EndAt, &ad.Impressions, &ad.Clicks,
		&ad.ReviewedBy, &ad.ReviewedAt, &ad.ReviewNote,
		&ad.CreatedAt, &ad.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ad get: %w", err)
	}
	return ad, nil
}

func (r *monetizationRepository) CreateAd(
	ctx context.Context,
	advertiserID, title, body, imageURL, targetURL, phoneNumber, whatsappNumber, status string,
	startAt, endAt *time.Time,
	weight int,
	dailyImpressionCap *int,
	targetProvinces, targetLanguages []string,
) (*models.Ad, error) {
	if weight <= 0 {
		weight = 1
	}
	if targetProvinces == nil {
		targetProvinces = []string{}
	}
	if targetLanguages == nil {
		targetLanguages = []string{}
	}
	const q = `
		INSERT INTO ads (
			advertiser_id, title, body, image_url, target_url,
			phone_number, whatsapp_number,
			weight, daily_impression_cap, target_provinces, target_languages,
			status, start_at, end_at
		)
		VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), $5,
			NULLIF($6, ''), NULLIF($7, ''),
			$8, $9, $10, $11,
			$12, $13, $14
		)
		RETURNING id
	`
	var id string
	if err := r.db.Pool.QueryRow(ctx, q,
		advertiserID, title, body, imageURL, targetURL,
		phoneNumber, whatsappNumber,
		weight, dailyImpressionCap, targetProvinces, targetLanguages,
		status, startAt, endAt,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("ad create: %w", err)
	}
	return r.GetAd(ctx, id)
}

func (r *monetizationRepository) UpdateAdStatus(
	ctx context.Context,
	id, status, reviewedBy string,
	req *models.AdReviewRequest,
) (*models.Ad, error) {
	const q = `
		UPDATE ads SET
			status      = $2,
			reviewed_by = $3,
			reviewed_at = NOW(),
			review_note = COALESCE($4, review_note),
			start_at    = COALESCE($5, start_at),
			end_at      = COALESCE($6, end_at),
			updated_at  = NOW()
		WHERE id = $1
	`
	tag, err := r.db.Pool.Exec(ctx, q, id, status, reviewedBy, req.Note, req.StartAt, req.EndAt)
	if err != nil {
		return nil, fmt.Errorf("ad update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return r.GetAd(ctx, id)
}

func (r *monetizationRepository) DeleteAd(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM ads WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("ad delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ─── Credits ─────────────────────────────────────────────────────────────────

func (r *monetizationRepository) ListBalances(ctx context.Context, search string, page, limit int) ([]*models.CreditBalance, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	args := []any{}
	where := ""
	if search != "" {
		where = "WHERE u.email ILIKE $1 OR up.first_name ILIKE $1 OR up.last_name ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := `
		SELECT COUNT(*) FROM credit_balances cb
		JOIN users u            ON u.id = cb.user_id
		LEFT JOIN profiles up ON up.id = cb.user_id
	` + " " + where
	if err := r.db.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("balances count: %w", err)
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT cb.user_id, COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), ''),
		       cb.balance, cb.updated_at
		FROM credit_balances cb
		JOIN users u            ON u.id = cb.user_id
		LEFT JOIN profiles up ON up.id = cb.user_id
		%s
		ORDER BY cb.balance DESC, cb.updated_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("balances list: %w", err)
	}
	defer rows.Close()

	var out []*models.CreditBalance
	for rows.Next() {
		b := &models.CreditBalance{}
		if err := rows.Scan(&b.UserID, &b.Email, &b.FullName, &b.Balance, &b.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("balance scan: %w", err)
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

func (r *monetizationRepository) GetBalance(ctx context.Context, userID string) (*models.CreditBalance, error) {
	const q = `
		SELECT cb.user_id, COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), ''),
		       cb.balance, cb.updated_at
		FROM credit_balances cb
		JOIN users u            ON u.id = cb.user_id
		LEFT JOIN profiles up ON up.id = cb.user_id
		WHERE cb.user_id = $1
	`
	b := &models.CreditBalance{}
	err := r.db.Pool.QueryRow(ctx, q, userID).Scan(
		&b.UserID, &b.Email, &b.FullName, &b.Balance, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Return a zero-balance placeholder so the admin UI can still render
		// the user's identity and offer an adjust action.
		var email, fullName string
		err2 := r.db.Pool.QueryRow(ctx, `
			SELECT COALESCE(u.email, ''),
			       COALESCE(NULLIF(TRIM(CONCAT_WS(' ', up.first_name, up.last_name)), ''), '')
			FROM users u
			LEFT JOIN profiles up ON up.id = u.id
			WHERE u.id = $1
		`, userID).Scan(&email, &fullName)
		if err2 != nil {
			return nil, nil
		}
		return &models.CreditBalance{UserID: userID, Email: email, FullName: fullName, Balance: 0}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("balance get: %w", err)
	}
	return b, nil
}

func (r *monetizationRepository) ListUserTransactions(ctx context.Context, userID string, limit int) ([]*models.CreditTransaction, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	const q = `
		SELECT t.id, t.user_id, t.amount, t.type, t.reason, t.note,
		       t.admin_id::text, u.email, t.created_at
		FROM credit_transactions t
		LEFT JOIN users u ON u.id = t.admin_id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("tx list: %w", err)
	}
	defer rows.Close()

	var out []*models.CreditTransaction
	for rows.Next() {
		tx := &models.CreditTransaction{}
		if err := rows.Scan(
			&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Reason, &tx.Note,
			&tx.AdminID, &tx.AdminEmail, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("tx scan: %w", err)
		}
		out = append(out, tx)
	}
	return out, rows.Err()
}

// AdjustCredits applies an admin-driven delta in a single transaction, so the
// ledger row and the cached balance can never drift. Refuses to drive the
// balance negative (the constraint is also enforced at the table level, but
// failing here yields a friendlier error).
func (r *monetizationRepository) AdjustCredits(
	ctx context.Context,
	userID string,
	req *models.AdjustCreditsRequest,
	adminID string,
) (*models.CreditBalance, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("credits begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txType := "ADJUST_ADD"
	if req.Amount < 0 {
		txType = "ADJUST_REMOVE"
	}

	// Upsert balance row first so the FK on the ledger is always satisfied.
	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_balances (user_id, balance)
		VALUES ($1, GREATEST(0, $2))
		ON CONFLICT (user_id) DO UPDATE
		SET balance    = credit_balances.balance + EXCLUDED.balance,
		    updated_at = NOW()
	`, userID, req.Amount); err != nil {
		// Likely a CHECK violation when balance would go negative.
		return nil, fmt.Errorf("credits balance update: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_transactions (user_id, amount, type, reason, note, admin_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, req.Amount, txType, req.Reason, req.Note, adminID); err != nil {
		return nil, fmt.Errorf("credits ledger insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("credits commit: %w", err)
	}
	return r.GetBalance(ctx, userID)
}

// ─── Boosts ──────────────────────────────────────────────────────────────────

func (r *monetizationRepository) ListBoosts(ctx context.Context, status string, page, limit int) ([]*models.Boost, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	args := []any{}
	where := ""
	if status != "" {
		where = "WHERE b.status = $1"
		args = append(args, status)
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM boosts b "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("boosts count: %w", err)
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT b.id, b.post_id, COALESCE(p.title, COALESCE(p.description, '')),
		       b.user_id, COALESCE(u.email, ''),
		       b.status, b.started_at, b.expires_at, b.credits_spent,
		       b.cancelled_by::text, b.cancelled_at, b.cancel_reason, b.created_at
		FROM boosts b
		LEFT JOIN posts p ON p.id = b.post_id
		LEFT JOIN users u ON u.id = b.user_id
		%s
		ORDER BY b.started_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("boosts list: %w", err)
	}
	defer rows.Close()

	var out []*models.Boost
	for rows.Next() {
		b := &models.Boost{}
		if err := rows.Scan(
			&b.ID, &b.PostID, &b.PostTitle, &b.UserID, &b.UserEmail,
			&b.Status, &b.StartedAt, &b.ExpiresAt, &b.CreditsSpent,
			&b.CancelledBy, &b.CancelledAt, &b.CancelReason, &b.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("boost scan: %w", err)
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

func (r *monetizationRepository) GetBoost(ctx context.Context, id string) (*models.Boost, error) {
	const q = `
		SELECT b.id, b.post_id, COALESCE(p.title, COALESCE(p.description, '')),
		       b.user_id, COALESCE(u.email, ''),
		       b.status, b.started_at, b.expires_at, b.credits_spent,
		       b.cancelled_by::text, b.cancelled_at, b.cancel_reason, b.created_at
		FROM boosts b
		LEFT JOIN posts p ON p.id = b.post_id
		LEFT JOIN users u ON u.id = b.user_id
		WHERE b.id = $1
	`
	b := &models.Boost{}
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&b.ID, &b.PostID, &b.PostTitle, &b.UserID, &b.UserEmail,
		&b.Status, &b.StartedAt, &b.ExpiresAt, &b.CreditsSpent,
		&b.CancelledBy, &b.CancelledAt, &b.CancelReason, &b.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("boost get: %w", err)
	}
	return b, nil
}

func (r *monetizationRepository) CancelBoost(ctx context.Context, id, adminID, reason string) (*models.Boost, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE boosts
		SET status        = 'CANCELLED',
		    cancelled_by  = $2,
		    cancelled_at  = NOW(),
		    cancel_reason = $3
		WHERE id = $1 AND status = 'ACTIVE'
	`, id, adminID, reason)
	if err != nil {
		return nil, fmt.Errorf("boost cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return r.GetBoost(ctx, id)
}

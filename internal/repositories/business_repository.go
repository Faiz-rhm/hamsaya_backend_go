package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// BusinessRepository defines the interface for business operations
type BusinessRepository interface {
	// Business Profile CRUD
	Create(ctx context.Context, business *models.BusinessProfile) error
	GetByID(ctx context.Context, businessID string) (*models.BusinessProfile, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.BusinessProfile, error)
	Update(ctx context.Context, business *models.BusinessProfile) error
	Delete(ctx context.Context, businessID string) error
	List(ctx context.Context, filter *models.BusinessListFilter) ([]*models.BusinessProfile, error)

	// Categories
	GetCategoriesByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessCategory, error)
	AddCategories(ctx context.Context, businessID string, categoryIDs []string) error
	RemoveCategories(ctx context.Context, businessID string) error

	// Business Hours
	GetHoursByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessHours, error)
	UpsertHours(ctx context.Context, hours *models.BusinessHours) error
	DeleteHoursByBusinessID(ctx context.Context, businessID string) error

	// Gallery
	AddAttachment(ctx context.Context, attachment *models.BusinessAttachment) error
	GetAttachmentsByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessAttachment, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error

	// Followers
	Follow(ctx context.Context, businessID, userID string) error
	Unfollow(ctx context.Context, businessID, userID string) error
	IsFollowing(ctx context.Context, businessID, userID string) (bool, error)
	GetFollowers(ctx context.Context, businessID string, limit, offset int) ([]string, error)

	// Categories Management
	GetAllCategories(ctx context.Context, search *string) ([]*models.BusinessCategory, error)
	// GetOrCreateCategoryByName returns category id by name; creates the category if it doesn't exist.
	GetOrCreateCategoryByName(ctx context.Context, name string) (string, error)
}

type businessRepository struct {
	db *database.DB
}

// NewBusinessRepository creates a new business repository
func NewBusinessRepository(db *database.DB) BusinessRepository {
	return &businessRepository{db: db}
}

// Create creates a new business profile
func (r *businessRepository) Create(ctx context.Context, business *models.BusinessProfile) error {
	// Use ST_SetSRID(ST_MakePoint()) for PostGIS geography column
	// pgtype.Point is not directly compatible with PostGIS geography type
	if business.AddressLocation != nil && business.AddressLocation.Valid {
		query := `
			INSERT INTO business_profiles (
				id, user_id, name, license_no, description, address, phone_number,
				email, website, avatar, cover, status, additional_info,
				address_location, country, province, district, neighborhood,
				show_location, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
				ST_SetSRID(ST_MakePoint($14, $15), 4326)::geography,
				$16, $17, $18, $19, $20, $21, $22)
		`
		_, err := r.db.Pool.Exec(ctx, query,
			business.ID,
			business.UserID,
			business.Name,
			business.LicenseNo,
			business.Description,
			business.Address,
			business.PhoneNumber,
			business.Email,
			business.Website,
			business.Avatar,
			business.Cover,
			business.Status,
			business.AdditionalInfo,
			business.AddressLocation.P.X, // longitude
			business.AddressLocation.P.Y, // latitude
			business.Country,
			business.Province,
			business.District,
			business.Neighborhood,
			business.ShowLocation,
			business.CreatedAt,
			business.UpdatedAt,
		)
		return err
	}

	query := `
		INSERT INTO business_profiles (
			id, user_id, name, license_no, description, address, phone_number,
			email, website, avatar, cover, status, additional_info,
			address_location, country, province, district, neighborhood,
			show_location, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULL, $14, $15, $16, $17, $18, $19, $20)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		business.ID,
		business.UserID,
		business.Name,
		business.LicenseNo,
		business.Description,
		business.Address,
		business.PhoneNumber,
		business.Email,
		business.Website,
		business.Avatar,
		business.Cover,
		business.Status,
		business.AdditionalInfo,
		business.Country,
		business.Province,
		business.District,
		business.Neighborhood,
		business.ShowLocation,
		business.CreatedAt,
		business.UpdatedAt,
	)

	return err
}

// GetByID gets a business profile by ID
// scanBusinessLocation scans PostGIS geography into pgtype.Point using ST_X/ST_Y
func scanBusinessLocation(lng, lat *float64, business *models.BusinessProfile) {
	if lng != nil && lat != nil {
		business.AddressLocation = &pgtype.Point{
			P:     pgtype.Vec2{X: *lng, Y: *lat},
			Valid: true,
		}
	}
}

func (r *businessRepository) GetByID(ctx context.Context, businessID string) (*models.BusinessProfile, error) {
	query := `
		SELECT
			id, user_id, name, license_no, description, address, phone_number,
			email, website, avatar, cover, status, additional_info,
			ST_X(address_location::geometry), ST_Y(address_location::geometry),
			country, province, district, neighborhood,
			show_location, total_views, total_follow, created_at, updated_at
		FROM business_profiles
		WHERE id = $1 AND deleted_at IS NULL
	`

	business := &models.BusinessProfile{}
	var lng, lat *float64
	err := r.db.Pool.QueryRow(ctx, query, businessID).Scan(
		&business.ID,
		&business.UserID,
		&business.Name,
		&business.LicenseNo,
		&business.Description,
		&business.Address,
		&business.PhoneNumber,
		&business.Email,
		&business.Website,
		&business.Avatar,
		&business.Cover,
		&business.Status,
		&business.AdditionalInfo,
		&lng,
		&lat,
		&business.Country,
		&business.Province,
		&business.District,
		&business.Neighborhood,
		&business.ShowLocation,
		&business.TotalViews,
		&business.TotalFollow,
		&business.CreatedAt,
		&business.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("business profile not found")
	}
	if err == nil {
		scanBusinessLocation(lng, lat, business)
	}

	return business, err
}

// GetByUserID gets all business profiles for a user
func (r *businessRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*models.BusinessProfile, error) {
	query := `
		SELECT
			id, user_id, name, license_no, description, address, phone_number,
			email, website, avatar, cover, status, additional_info,
			ST_X(address_location::geometry), ST_Y(address_location::geometry),
			country, province, district, neighborhood,
			show_location, total_views, total_follow, created_at, updated_at
		FROM business_profiles
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var businesses []*models.BusinessProfile
	for rows.Next() {
		business := &models.BusinessProfile{}
		var lng, lat *float64
		err := rows.Scan(
			&business.ID,
			&business.UserID,
			&business.Name,
			&business.LicenseNo,
			&business.Description,
			&business.Address,
			&business.PhoneNumber,
			&business.Email,
			&business.Website,
			&business.Avatar,
			&business.Cover,
			&business.Status,
			&business.AdditionalInfo,
			&lng,
			&lat,
			&business.Country,
			&business.Province,
			&business.District,
			&business.Neighborhood,
			&business.ShowLocation,
			&business.TotalViews,
			&business.TotalFollow,
			&business.CreatedAt,
			&business.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		scanBusinessLocation(lng, lat, business)
		businesses = append(businesses, business)
	}

	return businesses, rows.Err()
}

// Update updates a business profile
func (r *businessRepository) Update(ctx context.Context, business *models.BusinessProfile) error {
	// Use ST_SetSRID(ST_MakePoint()) for PostGIS geography column
	if business.AddressLocation != nil && business.AddressLocation.Valid {
		query := `
			UPDATE business_profiles
			SET
				name = $2,
				license_no = $3,
				description = $4,
				address = $5,
				phone_number = $6,
				email = $7,
				website = $8,
				avatar = $9,
				cover = $10,
				status = $11,
				additional_info = $12,
				address_location = ST_SetSRID(ST_MakePoint($13, $14), 4326)::geography,
				country = $15,
				province = $16,
				district = $17,
				neighborhood = $18,
				show_location = $19,
				updated_at = $20
			WHERE id = $1 AND deleted_at IS NULL
		`
		_, err := r.db.Pool.Exec(ctx, query,
			business.ID,
			business.Name,
			business.LicenseNo,
			business.Description,
			business.Address,
			business.PhoneNumber,
			business.Email,
			business.Website,
			business.Avatar,
			business.Cover,
			business.Status,
			business.AdditionalInfo,
			business.AddressLocation.P.X, // longitude
			business.AddressLocation.P.Y, // latitude
			business.Country,
			business.Province,
			business.District,
			business.Neighborhood,
			business.ShowLocation,
			business.UpdatedAt,
		)
		return err
	}

	query := `
		UPDATE business_profiles
		SET
			name = $2,
			license_no = $3,
			description = $4,
			address = $5,
			phone_number = $6,
			email = $7,
			website = $8,
			avatar = $9,
			cover = $10,
			status = $11,
			additional_info = $12,
			address_location = NULL,
			country = $13,
			province = $14,
			district = $15,
			neighborhood = $16,
			show_location = $17,
			updated_at = $18
		WHERE id = $1 AND deleted_at IS NULL
	`
	_, err := r.db.Pool.Exec(ctx, query,
		business.ID,
		business.Name,
		business.LicenseNo,
		business.Description,
		business.Address,
		business.PhoneNumber,
		business.Email,
		business.Website,
		business.Avatar,
		business.Cover,
		business.Status,
		business.AdditionalInfo,
		business.Country,
		business.Province,
		business.District,
		business.Neighborhood,
		business.ShowLocation,
		business.UpdatedAt,
	)

	return err
}

// Delete soft deletes a business profile
func (r *businessRepository) Delete(ctx context.Context, businessID string) error {
	query := `
		UPDATE business_profiles
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, businessID, time.Now())
	return err
}

// List lists business profiles with filters
func (r *businessRepository) List(ctx context.Context, filter *models.BusinessListFilter) ([]*models.BusinessProfile, error) {
	query := `
		SELECT DISTINCT
			bp.id, bp.user_id, bp.name, bp.license_no, bp.description, bp.address,
			bp.phone_number, bp.email, bp.website, bp.avatar, bp.cover, bp.status,
			bp.additional_info, ST_X(bp.address_location::geometry), ST_Y(bp.address_location::geometry),
			bp.country, bp.province,
			bp.district, bp.neighborhood, bp.show_location, bp.total_views,
			bp.total_follow, bp.created_at, bp.updated_at
		FROM business_profiles bp
	`

	var conditions []string
	var args []interface{}
	argCount := 1

	conditions = append(conditions, "bp.deleted_at IS NULL")

	if filter.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("bp.user_id = $%d", argCount))
		args = append(args, *filter.UserID)
		argCount++
	}

	if filter.CategoryID != nil {
		query += " INNER JOIN business_profile_categories bpc ON bp.id = bpc.business_profile_id"
		conditions = append(conditions, fmt.Sprintf("bpc.business_category_id = $%d", argCount))
		args = append(args, *filter.CategoryID)
		argCount++
	}

	if filter.Province != nil {
		conditions = append(conditions, fmt.Sprintf("bp.province = $%d", argCount))
		args = append(args, *filter.Province)
		argCount++
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(bp.name ILIKE $%d OR bp.description ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+*filter.Search+"%")
		argCount++
	}

	if filter.Latitude != nil && filter.Longitude != nil && filter.RadiusKm != nil {
		conditions = append(conditions, fmt.Sprintf(`
			ST_DWithin(
				bp.address_location,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography,
				$%d
			)
		`, argCount, argCount+1, argCount+2))
		args = append(args, *filter.Longitude, *filter.Latitude, *filter.RadiusKm*1000) // Convert km to meters
		argCount += 3
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY bp.created_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var businesses []*models.BusinessProfile
	for rows.Next() {
		business := &models.BusinessProfile{}
		var lng, lat *float64
		err := rows.Scan(
			&business.ID,
			&business.UserID,
			&business.Name,
			&business.LicenseNo,
			&business.Description,
			&business.Address,
			&business.PhoneNumber,
			&business.Email,
			&business.Website,
			&business.Avatar,
			&business.Cover,
			&business.Status,
			&business.AdditionalInfo,
			&lng,
			&lat,
			&business.Country,
			&business.Province,
			&business.District,
			&business.Neighborhood,
			&business.ShowLocation,
			&business.TotalViews,
			&business.TotalFollow,
			&business.CreatedAt,
			&business.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		scanBusinessLocation(lng, lat, business)
		businesses = append(businesses, business)
	}

	return businesses, rows.Err()
}

// GetCategoriesByBusinessID gets all categories for a business
func (r *businessRepository) GetCategoriesByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessCategory, error) {
	query := `
		SELECT bc.id, bc.name, bc.is_active, bc.created_at
		FROM business_categories bc
		INNER JOIN business_profile_categories bpc ON bc.id = bpc.business_category_id
		WHERE bpc.business_profile_id = $1
		ORDER BY bc.name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*models.BusinessCategory
	for rows.Next() {
		category := &models.BusinessCategory{}
		err := rows.Scan(&category.ID, &category.Name, &category.IsActive, &category.CreatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}

	return categories, rows.Err()
}

// AddCategories adds categories to a business
func (r *businessRepository) AddCategories(ctx context.Context, businessID string, categoryIDs []string) error {
	for _, categoryID := range categoryIDs {
		query := `
			INSERT INTO business_profile_categories (id, business_profile_id, business_category_id, created_at)
			VALUES (uuid_generate_v4(), $1, $2, NOW())
			ON CONFLICT (business_profile_id, business_category_id) DO NOTHING
		`
		_, err := r.db.Pool.Exec(ctx, query, businessID, categoryID)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveCategories removes all categories from a business
func (r *businessRepository) RemoveCategories(ctx context.Context, businessID string) error {
	query := `DELETE FROM business_profile_categories WHERE business_profile_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, businessID)
	return err
}

// GetHoursByBusinessID gets business hours for a business
func (r *businessRepository) GetHoursByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessHours, error) {
	query := `
		SELECT id, business_profile_id, day, open_time, close_time, is_closed, created_at, updated_at
		FROM business_hours
		WHERE business_profile_id = $1
		ORDER BY
			CASE day
				WHEN 'Monday' THEN 1
				WHEN 'Tuesday' THEN 2
				WHEN 'Wednesday' THEN 3
				WHEN 'Thursday' THEN 4
				WHEN 'Friday' THEN 5
				WHEN 'Saturday' THEN 6
				WHEN 'Sunday' THEN 7
			END
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hours []*models.BusinessHours
	for rows.Next() {
		hour := &models.BusinessHours{}
		var openTime, closeTime pgtype.Time
		err := rows.Scan(
			&hour.ID,
			&hour.BusinessProfileID,
			&hour.Day,
			&openTime,
			&closeTime,
			&hour.IsClosed,
			&hour.CreatedAt,
			&hour.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert pgtype.Time to *time.Time
		if openTime.Valid {
			t := time.Date(0, 1, 1, int(openTime.Microseconds/3600000000), int((openTime.Microseconds/60000000)%60), int((openTime.Microseconds/1000000)%60), 0, time.UTC)
			hour.OpenTime = &t
		}
		if closeTime.Valid {
			t := time.Date(0, 1, 1, int(closeTime.Microseconds/3600000000), int((closeTime.Microseconds/60000000)%60), int((closeTime.Microseconds/1000000)%60), 0, time.UTC)
			hour.CloseTime = &t
		}

		hours = append(hours, hour)
	}

	return hours, rows.Err()
}

// UpsertHours inserts or updates business hours
func (r *businessRepository) UpsertHours(ctx context.Context, hours *models.BusinessHours) error {
	query := `
		INSERT INTO business_hours (id, business_profile_id, day, open_time, close_time, is_closed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (business_profile_id, day)
		DO UPDATE SET
			open_time = EXCLUDED.open_time,
			close_time = EXCLUDED.close_time,
			is_closed = EXCLUDED.is_closed,
			updated_at = EXCLUDED.updated_at
	`

	// Convert *time.Time to pgtype.Time
	var openTime, closeTime pgtype.Time
	if hours.OpenTime != nil {
		openTime = pgtype.Time{
			Microseconds: int64(hours.OpenTime.Hour())*3600000000 + int64(hours.OpenTime.Minute())*60000000 + int64(hours.OpenTime.Second())*1000000,
			Valid:        true,
		}
	}
	if hours.CloseTime != nil {
		closeTime = pgtype.Time{
			Microseconds: int64(hours.CloseTime.Hour())*3600000000 + int64(hours.CloseTime.Minute())*60000000 + int64(hours.CloseTime.Second())*1000000,
			Valid:        true,
		}
	}

	_, err := r.db.Pool.Exec(ctx, query,
		hours.ID,
		hours.BusinessProfileID,
		hours.Day,
		openTime,
		closeTime,
		hours.IsClosed,
		hours.CreatedAt,
		hours.UpdatedAt,
	)

	return err
}

// DeleteHoursByBusinessID deletes all hours for a business
func (r *businessRepository) DeleteHoursByBusinessID(ctx context.Context, businessID string) error {
	query := `DELETE FROM business_hours WHERE business_profile_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, businessID)
	return err
}

// AddAttachment adds a gallery attachment
func (r *businessRepository) AddAttachment(ctx context.Context, attachment *models.BusinessAttachment) error {
	query := `
		INSERT INTO business_attachments (id, business_profile_id, photo, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		attachment.ID,
		attachment.BusinessProfileID,
		attachment.Photo,
		attachment.CreatedAt,
		attachment.UpdatedAt,
	)

	return err
}

// GetAttachmentsByBusinessID gets all gallery attachments for a business
func (r *businessRepository) GetAttachmentsByBusinessID(ctx context.Context, businessID string) ([]*models.BusinessAttachment, error) {
	query := `
		SELECT id, business_profile_id, photo, created_at, updated_at
		FROM business_attachments
		WHERE business_profile_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*models.BusinessAttachment
	for rows.Next() {
		attachment := &models.BusinessAttachment{}
		err := rows.Scan(
			&attachment.ID,
			&attachment.BusinessProfileID,
			&attachment.Photo,
			&attachment.CreatedAt,
			&attachment.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, rows.Err()
}

// DeleteAttachment soft deletes a gallery attachment
func (r *businessRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	query := `
		UPDATE business_attachments
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, attachmentID, time.Now())
	return err
}

// Follow follows a business
func (r *businessRepository) Follow(ctx context.Context, businessID, userID string) error {
	query := `
		INSERT INTO business_profile_followers (id, business_id, follower_id, is_active, created_at, updated_at)
		VALUES (uuid_generate_v4(), $1, $2, true, NOW(), NOW())
		ON CONFLICT (business_id, follower_id)
		DO UPDATE SET is_active = true, updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query, businessID, userID)
	return err
}

// Unfollow unfollows a business
func (r *businessRepository) Unfollow(ctx context.Context, businessID, userID string) error {
	query := `
		UPDATE business_profile_followers
		SET is_active = false, updated_at = NOW()
		WHERE business_id = $1 AND follower_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, businessID, userID)
	return err
}

// IsFollowing checks if a user is following a business
func (r *businessRepository) IsFollowing(ctx context.Context, businessID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM business_profile_followers
			WHERE business_id = $1 AND follower_id = $2 AND is_active = true
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, businessID, userID).Scan(&exists)
	return exists, err
}

// GetFollowers gets follower user IDs for a business
func (r *businessRepository) GetFollowers(ctx context.Context, businessID string, limit, offset int) ([]string, error) {
	query := `
		SELECT follower_id
		FROM business_profile_followers
		WHERE business_id = $1 AND is_active = true
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followerIDs []string
	for rows.Next() {
		var followerID string
		if err := rows.Scan(&followerID); err != nil {
			return nil, err
		}
		followerIDs = append(followerIDs, followerID)
	}

	return followerIDs, rows.Err()
}

// GetAllCategories gets all business categories, optionally filtered by search (name).
func (r *businessRepository) GetAllCategories(ctx context.Context, search *string) ([]*models.BusinessCategory, error) {
	query := `
		SELECT id, name, is_active, created_at
		FROM business_categories
		WHERE is_active = true
	`
	args := []interface{}{}
	if search != nil && *search != "" {
		query += ` AND name ILIKE '%' || $1 || '%'`
		args = append(args, *search)
	}
	query += ` ORDER BY name ASC`

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*models.BusinessCategory
	for rows.Next() {
		category := &models.BusinessCategory{}
		err := rows.Scan(&category.ID, &category.Name, &category.IsActive, &category.CreatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	if categories == nil {
		categories = []*models.BusinessCategory{}
	}
	return categories, rows.Err()
}

// GetOrCreateCategoryByName returns category id by name; creates the category if it doesn't exist.
func (r *businessRepository) GetOrCreateCategoryByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("category name is required")
	}
	var id string
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id FROM business_categories WHERE LOWER(name) = LOWER($1) AND is_active = true`,
		name,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	// Create new category
	err = r.db.Pool.QueryRow(ctx,
		`INSERT INTO business_categories (id, name, is_active, created_at)
		 VALUES (uuid_generate_v4(), $1, true, NOW())
		 RETURNING id`,
		name,
	).Scan(&id)
	return id, err
}

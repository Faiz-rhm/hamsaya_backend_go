package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// SearchRepository defines the interface for search operations
type SearchRepository interface {
	SearchPosts(ctx context.Context, filter *models.SearchFilter) ([]*models.Post, error)
	SearchUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.Profile, error)
	SearchBusinesses(ctx context.Context, filter *models.SearchFilter) ([]*models.BusinessProfile, error)
	GetDiscoverPosts(ctx context.Context, lat, lng, radiusKm float64, postType *models.PostType, limit int) ([]*models.Post, error)
	GetDiscoverBusinesses(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]*models.BusinessProfile, error)
}

type searchRepository struct {
	db *database.DB
}

// NewSearchRepository creates a new search repository
func NewSearchRepository(db *database.DB) SearchRepository {
	return &searchRepository{db: db}
}

// SearchPosts searches for posts using full-text search
func (r *searchRepository) SearchPosts(ctx context.Context, filter *models.SearchFilter) ([]*models.Post, error) {
	query := `
		SELECT DISTINCT p.*,
			ST_Y(p.address_location::geometry) as latitude,
			ST_X(p.address_location::geometry) as longitude
	`

	// Add distance calculation if location provided
	if filter.Latitude != nil && filter.Longitude != nil {
		query += fmt.Sprintf(`,
			ST_Distance(
				p.address_location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography
			) / 1000 as distance`, len(filter.Query)+3, len(filter.Query)+4)
	}

	query += `
		FROM posts p
		WHERE p.deleted_at IS NULL
			AND p.status = true
	`

	args := []interface{}{}
	argCount := 1

	// Full-text search on title and description
	if filter.Query != "" {
		searchTerm := "%" + strings.ToLower(filter.Query) + "%"
		query += fmt.Sprintf(`
			AND (
				LOWER(p.title) LIKE $%d
				OR LOWER(p.description) LIKE $%d
			)
		`, argCount, argCount)
		args = append(args, searchTerm)
		argCount++
	}

	// Location-based filtering
	if filter.Latitude != nil && filter.Longitude != nil && filter.RadiusKm != nil {
		query += fmt.Sprintf(`
			AND p.address_location IS NOT NULL
			AND ST_DWithin(
				p.address_location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography,
				$%d
			)
		`, argCount, argCount+1, argCount+2)
		args = append(args, *filter.Longitude, *filter.Latitude, *filter.RadiusKm*1000)
		argCount += 3
	}

	// Order by relevance and recency
	if filter.Latitude != nil && filter.Longitude != nil {
		query += ` ORDER BY distance ASC, p.created_at DESC`
	} else {
		query += ` ORDER BY p.created_at DESC`
	}

	// Pagination
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argCount, argCount+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search posts: %w", err)
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		var lat, lng *float64
		var distance *float64

		scanArgs := []interface{}{
			&post.ID,
			&post.UserID,
			&post.BusinessID,
			&post.OriginalPostID,
			&post.CategoryID,
			&post.Title,
			&post.Description,
			&post.Type,
			&post.Status,
			&post.Visibility,
			&post.Currency,
			&post.Price,
			&post.Discount,
			&post.Free,
			&post.Sold,
			&post.IsPromoted,
			&post.CountryCode,
			&post.ContactNo,
			&post.IsLocation,
			&post.StartDate,
			&post.StartTime,
			&post.EndDate,
			&post.EndTime,
			&post.EventState,
			&post.InterestedCount,
			&post.GoingCount,
			&post.ExpiredAt,
			&post.AddressLocation,
			&post.UserLocation,
			&post.Country,
			&post.Province,
			&post.District,
			&post.Neighborhood,
			&post.TotalComments,
			&post.TotalLikes,
			&post.TotalShares,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.DeletedAt,
			&lat,
			&lng,
		}

		if filter.Latitude != nil && filter.Longitude != nil {
			scanArgs = append(scanArgs, &distance)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}

	return posts, nil
}

// SearchUsers searches for users using full-text search
func (r *searchRepository) SearchUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.Profile, error) {
	query := `
		SELECT p.*, u.email,
			ST_Y(p.location::geometry) as latitude,
			ST_X(p.location::geometry) as longitude,
			(SELECT COUNT(*) FROM user_follows WHERE following_id = p.id) as follower_count,
			(SELECT COUNT(*) FROM user_follows WHERE follower_id = p.id) as following_count
		FROM profiles p
		JOIN users u ON u.id = p.id
		WHERE p.deleted_at IS NULL
			AND u.deleted_at IS NULL
	`

	args := []interface{}{}
	argCount := 1

	// Full-text search on name
	if filter.Query != "" {
		searchTerm := "%" + strings.ToLower(filter.Query) + "%"
		query += fmt.Sprintf(`
			AND (
				LOWER(p.first_name) LIKE $%d
				OR LOWER(p.last_name) LIKE $%d
				OR LOWER(CONCAT(p.first_name, ' ', p.last_name)) LIKE $%d
			)
		`, argCount, argCount, argCount)
		args = append(args, searchTerm)
		argCount++
	}

	// Location-based filtering
	if filter.Latitude != nil && filter.Longitude != nil && filter.RadiusKm != nil {
		query += fmt.Sprintf(`
			AND p.location IS NOT NULL
			AND ST_DWithin(
				p.location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography,
				$%d
			)
		`, argCount, argCount+1, argCount+2)
		args = append(args, *filter.Longitude, *filter.Latitude, *filter.RadiusKm*1000)
		argCount += 3
	}

	// Order by follower count for relevance
	query += ` ORDER BY follower_count DESC`

	// Pagination
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argCount, argCount+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		profile := &models.Profile{}
		var email string
		var lat, lng *float64
		var followerCount, followingCount int

		err := rows.Scan(
			&profile.ID,
			&profile.FirstName,
			&profile.LastName,
			&profile.Avatar,
			&profile.Cover,
			&profile.About,
			&profile.Gender,
			&profile.DOB,
			&profile.Website,
			&profile.Location,
			&profile.Country,
			&profile.Province,
			&profile.District,
			&profile.Neighborhood,
			&profile.IsComplete,
			&profile.CreatedAt,
			&profile.UpdatedAt,
			&profile.DeletedAt,
			&email,
			&lat,
			&lng,
			&followerCount,
			&followingCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan profile: %w", err)
		}

		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profiles: %w", err)
	}

	return profiles, nil
}

// SearchBusinesses searches for businesses using full-text search
func (r *searchRepository) SearchBusinesses(ctx context.Context, filter *models.SearchFilter) ([]*models.BusinessProfile, error) {
	query := `
		SELECT DISTINCT bp.*,
			ST_Y(bp.address_location::geometry) as latitude,
			ST_X(bp.address_location::geometry) as longitude
	`

	// Add distance calculation if location provided
	if filter.Latitude != nil && filter.Longitude != nil {
		query += fmt.Sprintf(`,
			ST_Distance(
				bp.address_location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography
			) / 1000 as distance`, len(filter.Query)+3, len(filter.Query)+4)
	}

	query += `
		FROM business_profiles bp
		WHERE bp.deleted_at IS NULL
			AND bp.status = true
	`

	args := []interface{}{}
	argCount := 1

	// Full-text search on name and description
	if filter.Query != "" {
		searchTerm := "%" + strings.ToLower(filter.Query) + "%"
		query += fmt.Sprintf(`
			AND (
				LOWER(bp.name) LIKE $%d
				OR LOWER(bp.description) LIKE $%d
				OR LOWER(bp.address) LIKE $%d
			)
		`, argCount, argCount, argCount)
		args = append(args, searchTerm)
		argCount++
	}

	// Location-based filtering
	if filter.Latitude != nil && filter.Longitude != nil && filter.RadiusKm != nil {
		query += fmt.Sprintf(`
			AND bp.address_location IS NOT NULL
			AND ST_DWithin(
				bp.address_location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography,
				$%d
			)
		`, argCount, argCount+1, argCount+2)
		args = append(args, *filter.Longitude, *filter.Latitude, *filter.RadiusKm*1000)
		argCount += 3
	}

	// Order by relevance
	if filter.Latitude != nil && filter.Longitude != nil {
		query += ` ORDER BY distance ASC, bp.total_follow DESC`
	} else {
		query += ` ORDER BY bp.total_follow DESC`
	}

	// Pagination
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argCount, argCount+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search businesses: %w", err)
	}
	defer rows.Close()

	var businesses []*models.BusinessProfile
	for rows.Next() {
		business := &models.BusinessProfile{}
		var lat, lng *float64
		var distance *float64

		scanArgs := []interface{}{
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
			&business.AddressLocation,
			&business.Country,
			&business.Province,
			&business.District,
			&business.Neighborhood,
			&business.ShowLocation,
			&business.TotalViews,
			&business.TotalFollow,
			&business.CreatedAt,
			&business.UpdatedAt,
			&business.DeletedAt,
			&lat,
			&lng,
		}

		if filter.Latitude != nil && filter.Longitude != nil {
			scanArgs = append(scanArgs, &distance)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("failed to scan business: %w", err)
		}

		businesses = append(businesses, business)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating businesses: %w", err)
	}

	return businesses, nil
}

// GetDiscoverPosts gets posts within a radius for map discovery
func (r *searchRepository) GetDiscoverPosts(ctx context.Context, lat, lng, radiusKm float64, postType *models.PostType, limit int) ([]*models.Post, error) {
	query := `
		SELECT p.*,
			ST_Y(p.address_location::geometry) as latitude,
			ST_X(p.address_location::geometry) as longitude,
			ST_Distance(
				p.address_location::geography,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) / 1000 as distance
		FROM posts p
		WHERE p.deleted_at IS NULL
			AND p.status = true
			AND p.address_location IS NOT NULL
			AND ST_DWithin(
				p.address_location::geography,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
				$3
			)
	`

	args := []interface{}{lng, lat, radiusKm * 1000}

	if postType != nil {
		query += ` AND p.type = $4`
		args = append(args, *postType)
	}

	query += ` ORDER BY distance ASC`

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, len(args)+1)
		args = append(args, limit)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get discover posts: %w", err)
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		var lat, lng, distance *float64

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.BusinessID,
			&post.OriginalPostID,
			&post.CategoryID,
			&post.Title,
			&post.Description,
			&post.Type,
			&post.Status,
			&post.Visibility,
			&post.Currency,
			&post.Price,
			&post.Discount,
			&post.Free,
			&post.Sold,
			&post.IsPromoted,
			&post.CountryCode,
			&post.ContactNo,
			&post.IsLocation,
			&post.StartDate,
			&post.StartTime,
			&post.EndDate,
			&post.EndTime,
			&post.EventState,
			&post.InterestedCount,
			&post.GoingCount,
			&post.ExpiredAt,
			&post.AddressLocation,
			&post.UserLocation,
			&post.Country,
			&post.Province,
			&post.District,
			&post.Neighborhood,
			&post.TotalComments,
			&post.TotalLikes,
			&post.TotalShares,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.DeletedAt,
			&lat,
			&lng,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}

	return posts, nil
}

// GetDiscoverBusinesses gets businesses within a radius for map discovery
func (r *searchRepository) GetDiscoverBusinesses(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]*models.BusinessProfile, error) {
	query := `
		SELECT bp.*,
			ST_Y(bp.address_location::geometry) as latitude,
			ST_X(bp.address_location::geometry) as longitude,
			ST_Distance(
				bp.address_location::geography,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			) / 1000 as distance
		FROM business_profiles bp
		WHERE bp.deleted_at IS NULL
			AND bp.status = true
			AND bp.address_location IS NOT NULL
			AND bp.show_location = true
			AND ST_DWithin(
				bp.address_location::geography,
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
				$3
			)
		ORDER BY distance ASC
		LIMIT $4
	`

	rows, err := r.db.Pool.Query(ctx, query, lng, lat, radiusKm*1000, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get discover businesses: %w", err)
	}
	defer rows.Close()

	var businesses []*models.BusinessProfile
	for rows.Next() {
		business := &models.BusinessProfile{}
		var lat, lng, distance *float64

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
			&business.AddressLocation,
			&business.Country,
			&business.Province,
			&business.District,
			&business.Neighborhood,
			&business.ShowLocation,
			&business.TotalViews,
			&business.TotalFollow,
			&business.CreatedAt,
			&business.UpdatedAt,
			&business.DeletedAt,
			&lat,
			&lng,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan business: %w", err)
		}

		businesses = append(businesses, business)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating businesses: %w", err)
	}

	return businesses, nil
}

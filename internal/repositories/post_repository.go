package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/jackc/pgx/v5"
)

// PostRepository defines the interface for post operations
type PostRepository interface {
	// Post CRUD
	Create(ctx context.Context, post *models.Post) error
	GetByID(ctx context.Context, postID string) (*models.Post, error)
	Update(ctx context.Context, post *models.Post) error
	Delete(ctx context.Context, postID string) error

	// Attachments
	CreateAttachment(ctx context.Context, attachment *models.Attachment) error
	GetAttachmentsByPostID(ctx context.Context, postID string) ([]*models.Attachment, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error

	// Likes
	LikePost(ctx context.Context, userID, postID string) error
	UnlikePost(ctx context.Context, userID, postID string) error
	IsLikedByUser(ctx context.Context, userID, postID string) (bool, error)
	GetPostLikes(ctx context.Context, postID string, limit, offset int) ([]*models.PostLike, error)

	// Bookmarks
	BookmarkPost(ctx context.Context, userID, postID string) error
	UnbookmarkPost(ctx context.Context, userID, postID string) error
	IsBookmarkedByUser(ctx context.Context, userID, postID string) (bool, error)
	GetUserBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error)

	// Shares
	SharePost(ctx context.Context, share *models.PostShare) error
	GetPostShares(ctx context.Context, postID string, limit, offset int) ([]*models.PostShare, error)

	// Feed
	GetFeed(ctx context.Context, filter *models.FeedFilter) ([]*models.Post, error)
	GetUserPosts(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error)
	GetBusinessPosts(ctx context.Context, businessID string, limit, offset int) ([]*models.Post, error)

	// Engagement status
	GetEngagementStatus(ctx context.Context, userID, postID string) (liked, bookmarked bool, err error)
}

type postRepository struct {
	db *database.DB
}

// NewPostRepository creates a new post repository
func NewPostRepository(db *database.DB) PostRepository {
	return &postRepository{db: db}
}

// Create creates a new post
func (r *postRepository) Create(ctx context.Context, post *models.Post) error {
	query := `
		INSERT INTO posts (
			id, user_id, business_id, original_post_id, category_id,
			title, description, type, status, visibility,
			currency, price, discount, free, sold, is_promoted, country_code, contact_no, is_location,
			start_date, start_time, end_date, end_time, event_state, interested_count, going_count, expired_at,
			address_location, user_location, country, province, district, neighborhood,
			total_comments, total_likes, total_shares,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25, $26, $27,
			$28, $29, $30, $31, $32, $33,
			$34, $35, $36,
			$37, $38
		)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		post.ID, post.UserID, post.BusinessID, post.OriginalPostID, post.CategoryID,
		post.Title, post.Description, post.Type, post.Status, post.Visibility,
		post.Currency, post.Price, post.Discount, post.Free, post.Sold, post.IsPromoted, post.CountryCode, post.ContactNo, post.IsLocation,
		post.StartDate, post.StartTime, post.EndDate, post.EndTime, post.EventState, post.InterestedCount, post.GoingCount, post.ExpiredAt,
		post.AddressLocation, post.UserLocation, post.Country, post.Province, post.District, post.Neighborhood,
		post.TotalComments, post.TotalLikes, post.TotalShares,
		post.CreatedAt, post.UpdatedAt,
	)

	return err
}

// GetByID gets a post by ID
func (r *postRepository) GetByID(ctx context.Context, postID string) (*models.Post, error) {
	query := `
		SELECT
			id, user_id, business_id, original_post_id, category_id,
			title, description, type, status, visibility,
			currency, price, discount, free, sold, is_promoted, country_code, contact_no, is_location,
			start_date, start_time, end_date, end_time, event_state, interested_count, going_count, expired_at,
			address_location, user_location, country, province, district, neighborhood,
			total_comments, total_likes, total_shares,
			created_at, updated_at, deleted_at
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL
	`

	post := &models.Post{}
	err := r.db.Pool.QueryRow(ctx, query, postID).Scan(
		&post.ID, &post.UserID, &post.BusinessID, &post.OriginalPostID, &post.CategoryID,
		&post.Title, &post.Description, &post.Type, &post.Status, &post.Visibility,
		&post.Currency, &post.Price, &post.Discount, &post.Free, &post.Sold, &post.IsPromoted, &post.CountryCode, &post.ContactNo, &post.IsLocation,
		&post.StartDate, &post.StartTime, &post.EndDate, &post.EndTime, &post.EventState, &post.InterestedCount, &post.GoingCount, &post.ExpiredAt,
		&post.AddressLocation, &post.UserLocation, &post.Country, &post.Province, &post.District, &post.Neighborhood,
		&post.TotalComments, &post.TotalLikes, &post.TotalShares,
		&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("post not found")
	}

	return post, err
}

// Update updates a post
func (r *postRepository) Update(ctx context.Context, post *models.Post) error {
	query := `
		UPDATE posts SET
			title = $2,
			description = $3,
			visibility = $4,
			price = $5,
			discount = $6,
			sold = $7,
			start_date = $8,
			start_time = $9,
			end_date = $10,
			end_time = $11,
			updated_at = $12
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query,
		post.ID,
		post.Title,
		post.Description,
		post.Visibility,
		post.Price,
		post.Discount,
		post.Sold,
		post.StartDate,
		post.StartTime,
		post.EndDate,
		post.EndTime,
		time.Now(),
	)

	return err
}

// Delete soft deletes a post
func (r *postRepository) Delete(ctx context.Context, postID string) error {
	query := `
		UPDATE posts
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, postID, time.Now())
	return err
}

// CreateAttachment creates a new attachment
func (r *postRepository) CreateAttachment(ctx context.Context, attachment *models.Attachment) error {
	query := `
		INSERT INTO attachments (id, post_id, photo, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		attachment.ID,
		attachment.PostID,
		attachment.Photo,
		attachment.CreatedAt,
		attachment.UpdatedAt,
	)

	return err
}

// GetAttachmentsByPostID gets all attachments for a post
func (r *postRepository) GetAttachmentsByPostID(ctx context.Context, postID string) ([]*models.Attachment, error) {
	query := `
		SELECT id, post_id, photo, created_at, updated_at
		FROM attachments
		WHERE post_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		attachment := &models.Attachment{}
		err := rows.Scan(
			&attachment.ID,
			&attachment.PostID,
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

// DeleteAttachment soft deletes an attachment
func (r *postRepository) DeleteAttachment(ctx context.Context, attachmentID string) error {
	query := `
		UPDATE attachments
		SET deleted_at = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.db.Pool.Exec(ctx, query, attachmentID, time.Now())
	return err
}

// LikePost likes a post (idempotent)
func (r *postRepository) LikePost(ctx context.Context, userID, postID string) error {
	query := `
		INSERT INTO post_likes (id, user_id, post_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, post_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		uuid.New().String(),
		userID,
		postID,
		time.Now(),
	)

	return err
}

// UnlikePost unlikes a post
func (r *postRepository) UnlikePost(ctx context.Context, userID, postID string) error {
	query := `
		DELETE FROM post_likes
		WHERE user_id = $1 AND post_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, postID)
	return err
}

// IsLikedByUser checks if a post is liked by a user
func (r *postRepository) IsLikedByUser(ctx context.Context, userID, postID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM post_likes
			WHERE user_id = $1 AND post_id = $2
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, postID).Scan(&exists)
	return exists, err
}

// GetPostLikes gets all likes for a post
func (r *postRepository) GetPostLikes(ctx context.Context, postID string, limit, offset int) ([]*models.PostLike, error) {
	query := `
		SELECT id, user_id, post_id, created_at
		FROM post_likes
		WHERE post_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var likes []*models.PostLike
	for rows.Next() {
		like := &models.PostLike{}
		err := rows.Scan(&like.ID, &like.UserID, &like.PostID, &like.CreatedAt)
		if err != nil {
			return nil, err
		}
		likes = append(likes, like)
	}

	return likes, rows.Err()
}

// BookmarkPost bookmarks a post (idempotent)
func (r *postRepository) BookmarkPost(ctx context.Context, userID, postID string) error {
	query := `
		INSERT INTO post_bookmarks (id, user_id, post_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, post_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		uuid.New().String(),
		userID,
		postID,
		time.Now(),
	)

	return err
}

// UnbookmarkPost removes a bookmark
func (r *postRepository) UnbookmarkPost(ctx context.Context, userID, postID string) error {
	query := `
		DELETE FROM post_bookmarks
		WHERE user_id = $1 AND post_id = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, postID)
	return err
}

// IsBookmarkedByUser checks if a post is bookmarked by a user
func (r *postRepository) IsBookmarkedByUser(ctx context.Context, userID, postID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM post_bookmarks
			WHERE user_id = $1 AND post_id = $2
		)
	`

	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, postID).Scan(&exists)
	return exists, err
}

// GetUserBookmarks gets all bookmarked posts for a user
func (r *postRepository) GetUserBookmarks(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error) {
	query := `
		SELECT
			p.id, p.user_id, p.business_id, p.original_post_id, p.category_id,
			p.title, p.description, p.type, p.status, p.visibility,
			p.currency, p.price, p.discount, p.free, p.sold, p.is_promoted, p.country_code, p.contact_no, p.is_location,
			p.start_date, p.start_time, p.end_date, p.end_time, p.event_state, p.interested_count, p.going_count, p.expired_at,
			p.address_location, p.user_location, p.country, p.province, p.district, p.neighborhood,
			p.total_comments, p.total_likes, p.total_shares,
			p.created_at, p.updated_at, p.deleted_at
		FROM posts p
		INNER JOIN post_bookmarks pb ON p.id = pb.post_id
		WHERE pb.user_id = $1 AND p.deleted_at IS NULL
		ORDER BY pb.created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryPosts(ctx, query, userID, limit, offset)
}

// SharePost creates a share record
func (r *postRepository) SharePost(ctx context.Context, share *models.PostShare) error {
	query := `
		INSERT INTO post_shares (id, user_id, original_post_id, shared_post_id, share_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		share.ID,
		share.UserID,
		share.OriginalPostID,
		share.SharedPostID,
		share.ShareText,
		share.CreatedAt,
	)

	return err
}

// GetPostShares gets all shares for a post
func (r *postRepository) GetPostShares(ctx context.Context, postID string, limit, offset int) ([]*models.PostShare, error) {
	query := `
		SELECT id, user_id, original_post_id, shared_post_id, share_text, created_at
		FROM post_shares
		WHERE original_post_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*models.PostShare
	for rows.Next() {
		share := &models.PostShare{}
		err := rows.Scan(
			&share.ID,
			&share.UserID,
			&share.OriginalPostID,
			&share.SharedPostID,
			&share.ShareText,
			&share.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}

	return shares, rows.Err()
}

// GetFeed gets posts based on filter criteria
func (r *postRepository) GetFeed(ctx context.Context, filter *models.FeedFilter) ([]*models.Post, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT
			id, user_id, business_id, original_post_id, category_id,
			title, description, type, status, visibility,
			currency, price, discount, free, sold, is_promoted, country_code, contact_no, is_location,
			start_date, start_time, end_date, end_time, event_state, interested_count, going_count, expired_at,
			address_location, user_location, country, province, district, neighborhood,
			total_comments, total_likes, total_shares,
			created_at, updated_at, deleted_at
		FROM posts
		WHERE deleted_at IS NULL AND status = true
	`)

	args := []interface{}{}
	argCount := 1

	// Apply filters
	if filter.Type != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND type = $%d", argCount))
		args = append(args, *filter.Type)
		argCount++
	}

	if filter.UserID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND user_id = $%d", argCount))
		args = append(args, *filter.UserID)
		argCount++
	}

	if filter.BusinessID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND business_id = $%d", argCount))
		args = append(args, *filter.BusinessID)
		argCount++
	}

	if filter.CategoryID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND category_id = $%d", argCount))
		args = append(args, *filter.CategoryID)
		argCount++
	}

	if filter.Province != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND province = $%d", argCount))
		args = append(args, *filter.Province)
		argCount++
	}

	// Location-based filtering (radius search)
	var locationSearchActive bool
	if filter.Latitude != nil && filter.Longitude != nil && filter.RadiusKm != nil {
		// PostGIS radius search: ST_DWithin expects geography and distance in meters
		queryBuilder.WriteString(fmt.Sprintf(`
			AND ST_DWithin(
				address_location::geography,
				ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography,
				$%d
			)
		`, argCount, argCount+1, argCount+2))
		args = append(args, *filter.Longitude, *filter.Latitude, *filter.RadiusKm*1000) // Convert km to meters
		argCount += 3
		locationSearchActive = true
	}

	// Sorting
	switch filter.SortBy {
	case "trending":
		// Trending score = (likes * 2 + comments * 3 + shares * 5) / age_hours^1.5
		queryBuilder.WriteString(`
			ORDER BY ((total_likes * 2 + total_comments * 3 + total_shares * 5) /
			POWER(EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 + 1, 1.5)) DESC
		`)
	case "nearby":
		// Distance-based sorting when location is provided
		if locationSearchActive && filter.Latitude != nil && filter.Longitude != nil {
			// Sort by distance (nearest first)
			queryBuilder.WriteString(fmt.Sprintf(`
				ORDER BY ST_Distance(
					address_location::geography,
					ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography
				) ASC
			`, argCount, argCount+1))
			args = append(args, *filter.Longitude, *filter.Latitude)
			argCount += 2
		} else {
			// Fallback to recent if no location provided
			queryBuilder.WriteString(" ORDER BY created_at DESC")
		}
	default: // recent
		queryBuilder.WriteString(" ORDER BY created_at DESC")
	}

	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1))
	args = append(args, filter.Limit, filter.Offset)

	return r.queryPosts(ctx, queryBuilder.String(), args...)
}

// GetUserPosts gets all posts by a user
func (r *postRepository) GetUserPosts(ctx context.Context, userID string, limit, offset int) ([]*models.Post, error) {
	query := `
		SELECT
			id, user_id, business_id, original_post_id, category_id,
			title, description, type, status, visibility,
			currency, price, discount, free, sold, is_promoted, country_code, contact_no, is_location,
			start_date, start_time, end_date, end_time, event_state, interested_count, going_count, expired_at,
			address_location, user_location, country, province, district, neighborhood,
			total_comments, total_likes, total_shares,
			created_at, updated_at, deleted_at
		FROM posts
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryPosts(ctx, query, userID, limit, offset)
}

// GetBusinessPosts gets all posts by a business
func (r *postRepository) GetBusinessPosts(ctx context.Context, businessID string, limit, offset int) ([]*models.Post, error) {
	query := `
		SELECT
			id, user_id, business_id, original_post_id, category_id,
			title, description, type, status, visibility,
			currency, price, discount, free, sold, is_promoted, country_code, contact_no, is_location,
			start_date, start_time, end_date, end_time, event_state, interested_count, going_count, expired_at,
			address_location, user_location, country, province, district, neighborhood,
			total_comments, total_likes, total_shares,
			created_at, updated_at, deleted_at
		FROM posts
		WHERE business_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryPosts(ctx, query, businessID, limit, offset)
}

// GetEngagementStatus gets like and bookmark status for a post
func (r *postRepository) GetEngagementStatus(ctx context.Context, userID, postID string) (liked, bookmarked bool, err error) {
	query := `
		SELECT
			EXISTS(SELECT 1 FROM post_likes WHERE user_id = $1 AND post_id = $2) AS liked,
			EXISTS(SELECT 1 FROM post_bookmarks WHERE user_id = $1 AND post_id = $2) AS bookmarked
	`

	err = r.db.Pool.QueryRow(ctx, query, userID, postID).Scan(&liked, &bookmarked)
	return
}

// queryPosts is a helper function to query posts
func (r *postRepository) queryPosts(ctx context.Context, query string, args ...interface{}) ([]*models.Post, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		err := rows.Scan(
			&post.ID, &post.UserID, &post.BusinessID, &post.OriginalPostID, &post.CategoryID,
			&post.Title, &post.Description, &post.Type, &post.Status, &post.Visibility,
			&post.Currency, &post.Price, &post.Discount, &post.Free, &post.Sold, &post.IsPromoted, &post.CountryCode, &post.ContactNo, &post.IsLocation,
			&post.StartDate, &post.StartTime, &post.EndDate, &post.EndTime, &post.EventState, &post.InterestedCount, &post.GoingCount, &post.ExpiredAt,
			&post.AddressLocation, &post.UserLocation, &post.Country, &post.Province, &post.District, &post.Neighborhood,
			&post.TotalComments, &post.TotalLikes, &post.TotalShares,
			&post.CreatedAt, &post.UpdatedAt, &post.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, rows.Err()
}

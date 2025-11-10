package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

// AdminRepository defines the interface for admin data operations
type AdminRepository interface {
	GetStatistics(ctx context.Context) (*models.AdminStatistics, error)
	ListUsers(ctx context.Context, search string, isActive *bool, page, limit int) ([]models.AdminUserListItem, int64, error)
	UpdateUserStatus(ctx context.Context, userID string, isActive bool) error
	UpdateUser(ctx context.Context, userID string, req *models.AdminUpdateUserRequest) error
	ListPosts(ctx context.Context, postType, search string, page, limit int) ([]models.AdminPostListItem, int64, error)
	UpdatePostStatus(ctx context.Context, postID string, status bool) error
	UpdatePost(ctx context.Context, postID string, req *models.AdminUpdatePostRequest) error
	ListReports(ctx context.Context, reportType, status, search string, page, limit int) ([]models.AdminReportListItem, int64, error)
	UpdateReportStatus(ctx context.Context, reportType, reportID, status string) error
	ListBusinesses(ctx context.Context, search string, status *bool, page, limit int) ([]models.AdminBusinessListItem, int64, error)
	UpdateBusinessStatus(ctx context.Context, businessID string, status bool) error
	UpdateBusiness(ctx context.Context, businessID string, req *models.AdminUpdateBusinessRequest) error
	GetSellPostStatistics(ctx context.Context) (*models.SellStatistics, error)
}

type adminRepository struct {
	db *database.DB
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *database.DB) AdminRepository {
	return &adminRepository{db: db}
}

// GetStatistics retrieves dashboard statistics for admin
func (r *adminRepository) GetStatistics(ctx context.Context) (*models.AdminStatistics, error) {
	stats := &models.AdminStatistics{}

	// Calculate start of current month for growth metrics
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Get total users count
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}

	// Get active users count (users who logged in within the last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE deleted_at IS NULL
		  AND last_login_at IS NOT NULL
		  AND last_login_at >= $1
	`, thirtyDaysAgo).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	// Calculate inactive users
	stats.InactiveUsers = stats.TotalUsers - stats.ActiveUsers

	// Get new users this month
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE deleted_at IS NULL
		  AND created_at >= $1
	`, startOfMonth).Scan(&stats.NewUsersThisMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get new users this month: %w", err)
	}

	// Get total posts count
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM posts
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalPosts)
	if err != nil {
		return nil, fmt.Errorf("failed to get total posts: %w", err)
	}

	// Get posts count by type
	rows, err := r.db.Pool.Query(ctx, `
		SELECT type, COUNT(*) as count
		FROM posts
		WHERE deleted_at IS NULL
		GROUP BY type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postType string
		var count int64
		if err := rows.Scan(&postType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan post type: %w", err)
		}

		switch postType {
		case "FEED":
			stats.PostsByType.Feed = count
		case "EVENT":
			stats.PostsByType.Event = count
		case "SELL":
			stats.PostsByType.Sell = count
		case "PULL":
			stats.PostsByType.Pull = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating posts by type: %w", err)
	}

	// Get new posts this month
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM posts
		WHERE deleted_at IS NULL
		  AND created_at >= $1
	`, startOfMonth).Scan(&stats.NewPostsThisMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get new posts this month: %w", err)
	}

	// Get total businesses count
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM business_profiles
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalBusinesses)
	if err != nil {
		return nil, fmt.Errorf("failed to get total businesses: %w", err)
	}

	// Get active businesses count
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM business_profiles
		WHERE deleted_at IS NULL
		  AND status = true
	`).Scan(&stats.ActiveBusinesses)
	if err != nil {
		return nil, fmt.Errorf("failed to get active businesses: %w", err)
	}

	// Get new businesses this month
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM business_profiles
		WHERE deleted_at IS NULL
		  AND created_at >= $1
	`, startOfMonth).Scan(&stats.NewBusinessesThisMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get new businesses this month: %w", err)
	}

	// Get engagement statistics
	// Total comments
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_comments
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalComments)
	if err != nil {
		return nil, fmt.Errorf("failed to get total comments: %w", err)
	}

	// Total likes (post_likes is a junction table with no deleted_at)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_likes
	`).Scan(&stats.TotalLikes)
	if err != nil {
		return nil, fmt.Errorf("failed to get total likes: %w", err)
	}

	// Total shares (sum of share counts from posts table)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_shares), 0)
		FROM posts
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalShares)
	if err != nil {
		return nil, fmt.Errorf("failed to get total shares: %w", err)
	}

	// Total bookmarks (post_bookmarks is a junction table with no deleted_at)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_bookmarks
	`).Scan(&stats.TotalBookmarks)
	if err != nil {
		return nil, fmt.Errorf("failed to get total bookmarks: %w", err)
	}

	// Get activity statistics
	// Total categories (sum of sell_categories and business_categories)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT (
			(SELECT COUNT(*) FROM sell_categories WHERE status = 'ACTIVE') +
			(SELECT COUNT(*) FROM business_categories WHERE is_active = true)
		) AS total_categories
	`).Scan(&stats.TotalCategories)
	if err != nil {
		return nil, fmt.Errorf("failed to get total categories: %w", err)
	}

	// Total poll votes (user_polls is a junction table with no deleted_at)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_polls
	`).Scan(&stats.TotalPollVotes)
	if err != nil {
		return nil, fmt.Errorf("failed to get total poll votes: %w", err)
	}

	// Total event interests (event_interests is a junction table with no deleted_at)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM event_interests
	`).Scan(&stats.TotalEventInterests)
	if err != nil {
		return nil, fmt.Errorf("failed to get total event interests: %w", err)
	}

	// Total follows (user_follows is a junction table with no deleted_at)
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_follows
	`).Scan(&stats.TotalFollows)
	if err != nil {
		return nil, fmt.Errorf("failed to get total follows: %w", err)
	}

	// Get pending reports counts
	// Post reports
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_reports
		WHERE report_status = 'PENDING'
	`).Scan(&stats.PendingReports.Posts)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending post reports: %w", err)
	}

	// Comment reports
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM comment_reports
		WHERE report_status = 'PENDING'
	`).Scan(&stats.PendingReports.Comments)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending comment reports: %w", err)
	}

	// User reports (note: uses 'resolved' field instead of 'report_status')
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_reports
		WHERE resolved = false
	`).Scan(&stats.PendingReports.Users)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending user reports: %w", err)
	}

	// Business reports
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM business_reports
		WHERE report_status = 'PENDING'
	`).Scan(&stats.PendingReports.Businesses)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending business reports: %w", err)
	}

	// Calculate total pending reports
	stats.PendingReports.Total = stats.PendingReports.Posts +
		stats.PendingReports.Comments +
		stats.PendingReports.Users +
		stats.PendingReports.Businesses

	// Get total reports count (all reports across all tables)
	var totalPostReports, totalCommentReports, totalUserReports, totalBusinessReports int64

	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM post_reports`).Scan(&totalPostReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get total post reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM comment_reports`).Scan(&totalCommentReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get total comment reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_reports`).Scan(&totalUserReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get total user reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM business_reports`).Scan(&totalBusinessReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get total business reports: %w", err)
	}

	stats.TotalReports = totalPostReports + totalCommentReports + totalUserReports + totalBusinessReports

	// Get resolved reports count
	var resolvedPostReports, resolvedCommentReports, resolvedUserReports, resolvedBusinessReports int64

	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM post_reports
		WHERE report_status = 'RESOLVED'
	`).Scan(&resolvedPostReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved post reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM comment_reports
		WHERE report_status = 'RESOLVED'
	`).Scan(&resolvedCommentReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved comment reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_reports
		WHERE resolved = true
	`).Scan(&resolvedUserReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved user reports: %w", err)
	}

	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM business_reports
		WHERE report_status = 'RESOLVED'
	`).Scan(&resolvedBusinessReports)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved business reports: %w", err)
	}

	stats.ResolvedReports = resolvedPostReports + resolvedCommentReports + resolvedUserReports + resolvedBusinessReports

	return stats, nil
}

// ListUsers retrieves a paginated list of users with optional filtering
func (r *adminRepository) ListUsers(ctx context.Context, search string, isActive *bool, page, limit int) ([]models.AdminUserListItem, int64, error) {
	// Build base query with dynamic WHERE clause
	baseQuery := `
		FROM users u
		LEFT JOIN profiles p ON u.id = p.id
		WHERE 1=1
	`

	// Build WHERE conditions
	var conditions []string
	var args []interface{}
	argCounter := 1

	// Filter by active status
	if isActive != nil {
		conditions = append(conditions, fmt.Sprintf("u.is_active = $%d", argCounter))
		args = append(args, *isActive)
		argCounter++
	}

	// Filter by search term
	if search != "" {
		searchPattern := "%" + search + "%"
		conditions = append(conditions, fmt.Sprintf(
			"(u.email ILIKE $%d OR p.first_name ILIKE $%d OR p.last_name ILIKE $%d)",
			argCounter, argCounter, argCounter,
		))
		args = append(args, searchPattern)
		argCounter++
	}

	// Combine conditions
	whereClause := baseQuery
	if len(conditions) > 0 {
		whereClause += " AND " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// Get total count
	var totalCount int64
	countQuery := "SELECT COUNT(*) " + whereClause
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total user count: %w", err)
	}

	// Get paginated users
	offset := (page - 1) * limit
	dataQuery := `
		SELECT
			u.id,
			u.email,
			p.first_name,
			p.last_name,
			u.email_verified,
			u.phone_verified,
			u.mfa_enabled,
			COALESCE(u.role::text, 'user') as role,
			u.is_active,
			u.last_login_at,
			u.created_at
	` + whereClause + `
		ORDER BY u.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argCounter) + ` OFFSET $` + fmt.Sprintf("%d", argCounter+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.AdminUserListItem
	for rows.Next() {
		var user models.AdminUserListItem
		var lastLoginAt *time.Time
		var createdAt time.Time

		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.FirstName,
			&user.LastName,
			&user.EmailVerified,
			&user.PhoneVerified,
			&user.MFAEnabled,
			&user.Role,
			&user.IsActive,
			&lastLoginAt,
			&createdAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}

		// Format timestamps
		if lastLoginAt != nil {
			lastLoginStr := lastLoginAt.Format(time.RFC3339)
			user.LastLoginAt = &lastLoginStr
		}
		createdAtStr := createdAt.Format(time.RFC3339)
		user.CreatedAt = createdAtStr

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating users: %w", err)
	}

	return users, totalCount, nil
}

// UpdateUserStatus updates a user's active status (soft delete/restore)
func (r *adminRepository) UpdateUserStatus(ctx context.Context, userID string, isActive bool) error {
	var query string
	if isActive {
		// Restore user by setting deleted_at to NULL
		query = `
			UPDATE users
			SET deleted_at = NULL, updated_at = NOW()
			WHERE id = $1
		`
	} else {
		// Soft delete user by setting deleted_at to current time
		query = `
			UPDATE users
			SET deleted_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`
	}

	result, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	return nil
}

// ListPosts retrieves a paginated list of posts with optional filtering
func (r *adminRepository) ListPosts(ctx context.Context, postType, search string, page, limit int) ([]models.AdminPostListItem, int64, error) {
	// Build base query with joins
	baseQuery := `
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		LEFT JOIN business_profiles b ON p.business_id = b.id
		WHERE p.deleted_at IS NULL
	`

	// Build WHERE conditions
	var conditions []string
	var args []interface{}
	argCounter := 1

	// Filter by post type
	if postType != "" && postType != "all" {
		conditions = append(conditions, fmt.Sprintf("p.type = $%d", argCounter))
		args = append(args, postType)
		argCounter++
	}

	// Filter by search term (title, description, user email, or business name)
	if search != "" {
		searchPattern := "%" + search + "%"
		conditions = append(conditions, fmt.Sprintf(
			"(p.title ILIKE $%d OR p.description ILIKE $%d OR u.email ILIKE $%d OR b.name ILIKE $%d)",
			argCounter, argCounter, argCounter, argCounter,
		))
		args = append(args, searchPattern)
		argCounter++
	}

	// Combine conditions
	whereClause := baseQuery
	if len(conditions) > 0 {
		whereClause += " AND " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// Get total count
	var totalCount int64
	countQuery := "SELECT COUNT(*) " + whereClause
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total post count: %w", err)
	}

	// Get paginated posts
	offset := (page - 1) * limit
	dataQuery := `
		SELECT
			p.id,
			p.user_id,
			u.email as user_email,
			CASE
				WHEN pr.first_name IS NOT NULL AND pr.last_name IS NOT NULL
				THEN pr.first_name || ' ' || pr.last_name
				WHEN pr.first_name IS NOT NULL THEN pr.first_name
				WHEN pr.last_name IS NOT NULL THEN pr.last_name
				ELSE NULL
			END as user_name,
			p.business_id,
			b.name as business_name,
			p.type,
			p.title,
			p.description,
			p.visibility,
			p.status,
			p.start_date,
			p.end_date,
			COALESCE(
				(
					SELECT jsonb_agg(a.photo)
					FROM attachments a
					WHERE a.post_id = p.id AND a.deleted_at IS NULL
				),
				'[]'::jsonb
			) as attachments,
			p.total_likes,
			p.total_comments,
			p.total_shares,
			p.created_at,
			p.updated_at
	` + whereClause + `
		ORDER BY p.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argCounter) + ` OFFSET $` + fmt.Sprintf("%d", argCounter+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list posts: %w", err)
	}
	defer rows.Close()

	var posts []models.AdminPostListItem
	for rows.Next() {
		var post models.AdminPostListItem
		var createdAt time.Time
		var updatedAt time.Time
		var startDate *time.Time
		var endDate *time.Time
		var attachmentsJSON []byte

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserEmail,
			&post.UserName,
			&post.BusinessID,
			&post.BusinessName,
			&post.Type,
			&post.Title,
			&post.Description,
			&post.Visibility,
			&post.Status,
			&startDate,
			&endDate,
			&attachmentsJSON,
			&post.TotalLikes,
			&post.TotalComments,
			&post.TotalShares,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan post: %w", err)
		}

		// Parse attachments JSON
		if len(attachmentsJSON) > 0 {
			if err := json.Unmarshal(attachmentsJSON, &post.Attachments); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal attachments: %w", err)
			}
		}
		if post.Attachments == nil {
			post.Attachments = []models.Photo{}
		}

		// Format timestamps
		createdAtStr := createdAt.Format(time.RFC3339)
		post.CreatedAt = createdAtStr
		updatedAtStr := updatedAt.Format(time.RFC3339)
		post.UpdatedAt = updatedAtStr

		// Format event dates (DATE type, not timestamp)
		if startDate != nil {
			startDateStr := startDate.Format("2006-01-02")
			post.StartDate = &startDateStr
		}
		if endDate != nil {
			endDateStr := endDate.Format("2006-01-02")
			post.EndDate = &endDateStr
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating posts: %w", err)
	}

	return posts, totalCount, nil
}

// ListReports retrieves a paginated list of reports from all report types
func (r *adminRepository) ListReports(ctx context.Context, reportType, status, search string, page, limit int) ([]models.AdminReportListItem, int64, error) {
	// Build UNION query to combine all report types
	var queries []string
	var args []interface{}
	argCounter := 1

	// Post Reports Query
	if reportType == "" || reportType == "all" || reportType == "POST" {
		postQuery := fmt.Sprintf(`
			SELECT
				pr.id,
				'POST' as report_type,
				pr.user_id as reporter_id,
				u.email as reporter_email,
				CASE
					WHEN p_prof.first_name IS NOT NULL AND p_prof.last_name IS NOT NULL
					THEN p_prof.first_name || ' ' || p_prof.last_name
					WHEN p_prof.first_name IS NOT NULL THEN p_prof.first_name
					WHEN p_prof.last_name IS NOT NULL THEN p_prof.last_name
					ELSE NULL
				END as reporter_name,
				pr.post_id as reported_item_id,
				COALESCE(p.title, LEFT(p.description, 50)) as reported_item_info,
				pr.reason,
				pr.additional_comments,
				pr.report_status::text as status,
				pr.created_at,
				pr.updated_at
			FROM post_reports pr
			LEFT JOIN users u ON pr.user_id = u.id
			LEFT JOIN profiles p_prof ON u.id = p_prof.id
			LEFT JOIN posts p ON pr.post_id = p.id
			WHERE 1=1
		`)
		queries = append(queries, postQuery)
	}

	// Comment Reports Query
	if reportType == "" || reportType == "all" || reportType == "COMMENT" {
		commentQuery := fmt.Sprintf(`
			SELECT
				cr.id,
				'COMMENT' as report_type,
				cr.user_id as reporter_id,
				u.email as reporter_email,
				CASE
					WHEN p_prof.first_name IS NOT NULL AND p_prof.last_name IS NOT NULL
					THEN p_prof.first_name || ' ' || p_prof.last_name
					WHEN p_prof.first_name IS NOT NULL THEN p_prof.first_name
					WHEN p_prof.last_name IS NOT NULL THEN p_prof.last_name
					ELSE NULL
				END as reporter_name,
				cr.comment_id as reported_item_id,
				LEFT(c.text, 50) as reported_item_info,
				cr.reason,
				cr.additional_comments,
				cr.report_status::text as status,
				cr.created_at,
				cr.updated_at
			FROM comment_reports cr
			LEFT JOIN users u ON cr.user_id = u.id
			LEFT JOIN profiles p_prof ON u.id = p_prof.id
			LEFT JOIN post_comments c ON cr.comment_id = c.id
			WHERE 1=1
		`)
		queries = append(queries, commentQuery)
	}

	// User Reports Query
	if reportType == "" || reportType == "all" || reportType == "USER" {
		userQuery := fmt.Sprintf(`
			SELECT
				ur.id,
				'USER' as report_type,
				ur.reported_by_id as reporter_id,
				u.email as reporter_email,
				CASE
					WHEN p_prof.first_name IS NOT NULL AND p_prof.last_name IS NOT NULL
					THEN p_prof.first_name || ' ' || p_prof.last_name
					WHEN p_prof.first_name IS NOT NULL THEN p_prof.first_name
					WHEN p_prof.last_name IS NOT NULL THEN p_prof.last_name
					ELSE NULL
				END as reporter_name,
				ur.reported_user as reported_item_id,
				CASE
					WHEN ru_prof.first_name IS NOT NULL AND ru_prof.last_name IS NOT NULL
					THEN ru_prof.first_name || ' ' || ru_prof.last_name
					WHEN ru_prof.first_name IS NOT NULL THEN ru_prof.first_name
					WHEN ru_prof.last_name IS NOT NULL THEN ru_prof.last_name
					ELSE ru.email
				END as reported_item_info,
				ur.reason,
				ur.description as additional_comments,
				CASE WHEN ur.resolved THEN 'RESOLVED' ELSE 'PENDING' END as status,
				ur.created_at,
				ur.updated_at
			FROM user_reports ur
			LEFT JOIN users u ON ur.reported_by_id = u.id
			LEFT JOIN profiles p_prof ON u.id = p_prof.id
			LEFT JOIN users ru ON ur.reported_user = ru.id
			LEFT JOIN profiles ru_prof ON ru.id = ru_prof.id
			WHERE 1=1
		`)
		queries = append(queries, userQuery)
	}

	// Business Reports Query
	if reportType == "" || reportType == "all" || reportType == "BUSINESS" {
		businessQuery := fmt.Sprintf(`
			SELECT
				br.id,
				'BUSINESS' as report_type,
				br.user_id as reporter_id,
				u.email as reporter_email,
				CASE
					WHEN p_prof.first_name IS NOT NULL AND p_prof.last_name IS NOT NULL
					THEN p_prof.first_name || ' ' || p_prof.last_name
					WHEN p_prof.first_name IS NOT NULL THEN p_prof.first_name
					WHEN p_prof.last_name IS NOT NULL THEN p_prof.last_name
					ELSE NULL
				END as reporter_name,
				br.business_id as reported_item_id,
				bp.name as reported_item_info,
				br.reason,
				br.additional_comments,
				br.report_status::text as status,
				br.created_at,
				br.updated_at
			FROM business_reports br
			LEFT JOIN users u ON br.user_id = u.id
			LEFT JOIN profiles p_prof ON u.id = p_prof.id
			LEFT JOIN business_profiles bp ON br.business_id = bp.id
			WHERE 1=1
		`)
		queries = append(queries, businessQuery)
	}

	// Combine queries with UNION ALL
	unionQuery := "(" + queries[0] + ")"
	for i := 1; i < len(queries); i++ {
		unionQuery += " UNION ALL (" + queries[i] + ")"
	}

	// Add filters to WHERE clause
	baseQuery := fmt.Sprintf(`
		WITH all_reports AS (
			%s
		)
		SELECT * FROM all_reports WHERE 1=1
	`, unionQuery)

	// Apply filters
	if reportType != "" && reportType != "all" {
		baseQuery += fmt.Sprintf(" AND report_type = $%d", argCounter)
		args = append(args, reportType)
		argCounter++
	}

	if status != "" && status != "all" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argCounter)
		args = append(args, status)
		argCounter++
	}

	if search != "" {
		searchPattern := "%" + search + "%"
		baseQuery += fmt.Sprintf(" AND (reporter_email ILIKE $%d OR reporter_name ILIKE $%d OR reported_item_info ILIKE $%d OR reason ILIKE $%d)",
			argCounter, argCounter, argCounter, argCounter)
		args = append(args, searchPattern)
		argCounter++
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS count_query", baseQuery)
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total report count: %w", err)
	}

	// Add pagination
	offset := (page - 1) * limit
	dataQuery := baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list reports: %w", err)
	}
	defer rows.Close()

	var reports []models.AdminReportListItem
	for rows.Next() {
		var report models.AdminReportListItem
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&report.ID,
			&report.ReportType,
			&report.ReporterID,
			&report.ReporterEmail,
			&report.ReporterName,
			&report.ReportedItemID,
			&report.ReportedItemInfo,
			&report.Reason,
			&report.AdditionalComments,
			&report.Status,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan report: %w", err)
		}

		// Format timestamps
		report.CreatedAt = createdAt.Format(time.RFC3339)
		report.UpdatedAt = updatedAt.Format(time.RFC3339)

		reports = append(reports, report)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating reports: %w", err)
	}

	return reports, totalCount, nil
}

// UpdateReportStatus updates the status of a report
func (r *adminRepository) UpdateReportStatus(ctx context.Context, reportType, reportID, status string) error {
	var query string
	var tableName string

	switch reportType {
	case "POST":
		tableName = "post_reports"
		query = fmt.Sprintf("UPDATE %s SET report_status = $1, updated_at = NOW() WHERE id = $2", tableName)
	case "COMMENT":
		tableName = "comment_reports"
		query = fmt.Sprintf("UPDATE %s SET report_status = $1, updated_at = NOW() WHERE id = $2", tableName)
	case "USER":
		tableName = "user_reports"
		// User reports use 'resolved' field instead of 'report_status'
		resolved := status == "RESOLVED"
		query = fmt.Sprintf("UPDATE %s SET resolved = $1, updated_at = NOW() WHERE id = $2", tableName)
		result, err := r.db.Pool.Exec(ctx, query, resolved, reportID)
		if err != nil {
			return fmt.Errorf("failed to update user report status: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("report not found: %s", reportID)
		}
		return nil
	case "BUSINESS":
		tableName = "business_reports"
		query = fmt.Sprintf("UPDATE %s SET report_status = $1, updated_at = NOW() WHERE id = $2", tableName)
	default:
		return fmt.Errorf("invalid report type: %s", reportType)
	}

	result, err := r.db.Pool.Exec(ctx, query, status, reportID)
	if err != nil {
		return fmt.Errorf("failed to update report status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("report not found: %s", reportID)
	}

	return nil
}

// ListBusinesses retrieves a paginated list of businesses with optional filtering
func (r *adminRepository) ListBusinesses(ctx context.Context, search string, status *bool, page, limit int) ([]models.AdminBusinessListItem, int64, error) {
	// Build base query with joins
	baseQuery := `
		FROM business_profiles bp
		LEFT JOIN users u ON bp.user_id = u.id
		LEFT JOIN profiles p ON u.id = p.id
		WHERE bp.deleted_at IS NULL
	`

	// Build WHERE conditions
	var conditions []string
	var args []interface{}
	argCounter := 1

	// Filter by status (active/inactive)
	if status != nil {
		conditions = append(conditions, fmt.Sprintf("bp.status = $%d", argCounter))
		args = append(args, *status)
		argCounter++
	}

	// Filter by search term (name, license_no, email, phone_number, province, district)
	if search != "" {
		searchPattern := "%" + search + "%"
		conditions = append(conditions, fmt.Sprintf(
			"(bp.name ILIKE $%d OR bp.license_no ILIKE $%d OR bp.email ILIKE $%d OR bp.phone_number ILIKE $%d OR bp.province ILIKE $%d OR bp.district ILIKE $%d OR u.email ILIKE $%d)",
			argCounter, argCounter, argCounter, argCounter, argCounter, argCounter, argCounter,
		))
		args = append(args, searchPattern)
		argCounter++
	}

	// Combine conditions
	whereClause := baseQuery
	if len(conditions) > 0 {
		whereClause += " AND " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// Get total count
	var totalCount int64
	countQuery := "SELECT COUNT(*) " + whereClause
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total business count: %w", err)
	}

	// Get paginated businesses
	offset := (page - 1) * limit
	dataQuery := `
		SELECT
			bp.id,
			bp.user_id,
			u.email as owner_email,
			CASE
				WHEN p.first_name IS NOT NULL AND p.last_name IS NOT NULL
				THEN p.first_name || ' ' || p.last_name
				WHEN p.first_name IS NOT NULL THEN p.first_name
				WHEN p.last_name IS NOT NULL THEN p.last_name
				ELSE NULL
			END as owner_name,
			bp.name,
			bp.license_no,
			bp.email,
			bp.phone_number,
			bp.province,
			bp.district,
			bp.status,
			bp.total_views,
			bp.total_follow,
			COALESCE((
				SELECT COUNT(*)
				FROM posts
				WHERE business_id = bp.id AND deleted_at IS NULL
			), 0) as total_posts,
			bp.created_at,
			bp.updated_at
	` + whereClause + `
		ORDER BY bp.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argCounter) + ` OFFSET $` + fmt.Sprintf("%d", argCounter+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list businesses: %w", err)
	}
	defer rows.Close()

	var businesses []models.AdminBusinessListItem
	for rows.Next() {
		var business models.AdminBusinessListItem
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&business.ID,
			&business.UserID,
			&business.OwnerEmail,
			&business.OwnerName,
			&business.Name,
			&business.LicenseNo,
			&business.Email,
			&business.PhoneNumber,
			&business.Province,
			&business.District,
			&business.Status,
			&business.TotalViews,
			&business.TotalFollow,
			&business.TotalPosts,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan business: %w", err)
		}

		// Format timestamps
		createdAtStr := createdAt.Format(time.RFC3339)
		business.CreatedAt = createdAtStr
		updatedAtStr := updatedAt.Format(time.RFC3339)
		business.UpdatedAt = updatedAtStr

		businesses = append(businesses, business)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating businesses: %w", err)
	}

	return businesses, totalCount, nil
}

// UpdateBusinessStatus updates a business's active status
func (r *adminRepository) UpdateBusinessStatus(ctx context.Context, businessID string, status bool) error {
	query := `
		UPDATE business_profiles
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, status, businessID)
	if err != nil {
		return fmt.Errorf("failed to update business status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("business not found: %s", businessID)
	}

	return nil
}

// UpdatePostStatus updates a post's active status
func (r *adminRepository) UpdatePostStatus(ctx context.Context, postID string, status bool) error {
	query := `
		UPDATE posts
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, status, postID)
	if err != nil {
		return fmt.Errorf("failed to update post status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("post not found: %s", postID)
	}

	return nil
}

// UpdateUser updates user information (admin operation)
func (r *adminRepository) UpdateUser(ctx context.Context, userID string, req *models.AdminUpdateUserRequest) error {
	// Build dynamic UPDATE query for users table
	var usersClauses []string
	var usersArgs []interface{}
	argCounter := 1

	if req.Email != nil {
		usersClauses = append(usersClauses, fmt.Sprintf("email = $%d", argCounter))
		usersArgs = append(usersArgs, *req.Email)
		argCounter++
	}

	if req.Role != nil {
		usersClauses = append(usersClauses, fmt.Sprintf("role = $%d::user_role", argCounter))
		usersArgs = append(usersArgs, *req.Role)
		argCounter++
	}

	if req.EmailVerified != nil {
		usersClauses = append(usersClauses, fmt.Sprintf("email_verified = $%d", argCounter))
		usersArgs = append(usersArgs, *req.EmailVerified)
		argCounter++
	}

	if req.PhoneVerified != nil {
		usersClauses = append(usersClauses, fmt.Sprintf("phone_verified = $%d", argCounter))
		usersArgs = append(usersArgs, *req.PhoneVerified)
		argCounter++
	}

	if req.IsActive != nil {
		if *req.IsActive {
			// Activate user by setting deleted_at to NULL
			usersClauses = append(usersClauses, fmt.Sprintf("deleted_at = NULL"))
		} else {
			// Deactivate user by setting deleted_at to NOW
			usersClauses = append(usersClauses, fmt.Sprintf("deleted_at = NOW()"))
		}
	}

	if req.MfaEnabled != nil {
		usersClauses = append(usersClauses, fmt.Sprintf("mfa_enabled = $%d", argCounter))
		usersArgs = append(usersArgs, *req.MfaEnabled)
		argCounter++
	}

	// Update users table if there are changes
	if len(usersClauses) > 0 {
		usersQuery := fmt.Sprintf(`
			UPDATE users
			SET %s, updated_at = NOW()
			WHERE id = $%d
		`, usersClauses[0], argCounter)

		for i := 1; i < len(usersClauses); i++ {
			usersQuery = fmt.Sprintf("UPDATE users SET %s, %s, updated_at = NOW() WHERE id = $%d",
				usersClauses[0], usersClauses[i], argCounter)
		}

		// Build the full query
		if len(usersClauses) == 1 {
			usersQuery = fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d",
				usersClauses[0], argCounter)
		} else {
			setClauses := ""
			for i, clause := range usersClauses {
				if i > 0 {
					setClauses += ", "
				}
				setClauses += clause
			}
			usersQuery = fmt.Sprintf("UPDATE users SET %s, updated_at = NOW() WHERE id = $%d",
				setClauses, argCounter)
		}

		usersArgs = append(usersArgs, userID)

		result, err := r.db.Pool.Exec(ctx, usersQuery, usersArgs...)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		if result.RowsAffected() == 0 {
			return fmt.Errorf("user not found: %s", userID)
		}
	}

	// Build dynamic UPDATE query for profiles table
	var profilesClauses []string
	var profilesArgs []interface{}
	argCounter = 1

	if req.FirstName != nil {
		profilesClauses = append(profilesClauses, fmt.Sprintf("first_name = $%d", argCounter))
		profilesArgs = append(profilesArgs, *req.FirstName)
		argCounter++
	}

	if req.LastName != nil {
		profilesClauses = append(profilesClauses, fmt.Sprintf("last_name = $%d", argCounter))
		profilesArgs = append(profilesArgs, *req.LastName)
		argCounter++
	}

	// Update profiles table if there are changes
	if len(profilesClauses) > 0 {
		setClauses := ""
		for i, clause := range profilesClauses {
			if i > 0 {
				setClauses += ", "
			}
			setClauses += clause
		}

		profilesQuery := fmt.Sprintf(`
			UPDATE profiles
			SET %s, updated_at = NOW()
			WHERE id = $%d
		`, setClauses, argCounter)

		profilesArgs = append(profilesArgs, userID)

		_, err := r.db.Pool.Exec(ctx, profilesQuery, profilesArgs...)
		if err != nil {
			return fmt.Errorf("failed to update user profile: %w", err)
		}
	}

	return nil
}

// UpdatePost updates post information (admin operation)
func (r *adminRepository) UpdatePost(ctx context.Context, postID string, req *models.AdminUpdatePostRequest) error {
	// Build dynamic UPDATE query
	var clauses []string
	var args []interface{}
	argCounter := 1

	if req.Title != nil {
		clauses = append(clauses, fmt.Sprintf("title = $%d", argCounter))
		args = append(args, *req.Title)
		argCounter++
	}

	if req.Description != nil {
		clauses = append(clauses, fmt.Sprintf("description = $%d", argCounter))
		args = append(args, *req.Description)
		argCounter++
	}

	if req.Visibility != nil {
		clauses = append(clauses, fmt.Sprintf("visibility = $%d", argCounter))
		args = append(args, *req.Visibility)
		argCounter++
	}

	if req.Type != nil {
		clauses = append(clauses, fmt.Sprintf("type = $%d", argCounter))
		args = append(args, *req.Type)
		argCounter++
	}

	if req.Status != nil {
		clauses = append(clauses, fmt.Sprintf("status = $%d", argCounter))
		args = append(args, *req.Status)
		argCounter++
	}

	if req.StartDate != nil {
		clauses = append(clauses, fmt.Sprintf("start_date = $%d", argCounter))
		args = append(args, *req.StartDate)
		argCounter++
	}

	if req.EndDate != nil {
		clauses = append(clauses, fmt.Sprintf("end_date = $%d", argCounter))
		args = append(args, *req.EndDate)
		argCounter++
	}

	// If no fields to update, return error
	if len(clauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Build SET clause
	setClauses := ""
	for i, clause := range clauses {
		if i > 0 {
			setClauses += ", "
		}
		setClauses += clause
	}

	query := fmt.Sprintf(`
		UPDATE posts
		SET %s, updated_at = NOW()
		WHERE id = $%d AND deleted_at IS NULL
	`, setClauses, argCounter)

	args = append(args, postID)

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("post not found: %s", postID)
	}

	return nil
}

// UpdateBusiness updates business information (admin operation)
func (r *adminRepository) UpdateBusiness(ctx context.Context, businessID string, req *models.AdminUpdateBusinessRequest) error {
	// Build dynamic UPDATE query
	var clauses []string
	var args []interface{}
	argCounter := 1

	if req.Name != nil {
		clauses = append(clauses, fmt.Sprintf("name = $%d", argCounter))
		args = append(args, *req.Name)
		argCounter++
	}

	if req.LicenseNo != nil {
		clauses = append(clauses, fmt.Sprintf("license_no = $%d", argCounter))
		args = append(args, *req.LicenseNo)
		argCounter++
	}

	if req.Email != nil {
		clauses = append(clauses, fmt.Sprintf("email = $%d", argCounter))
		args = append(args, *req.Email)
		argCounter++
	}

	if req.PhoneNumber != nil {
		clauses = append(clauses, fmt.Sprintf("phone_number = $%d", argCounter))
		args = append(args, *req.PhoneNumber)
		argCounter++
	}

	if req.Province != nil {
		clauses = append(clauses, fmt.Sprintf("province = $%d", argCounter))
		args = append(args, *req.Province)
		argCounter++
	}

	if req.District != nil {
		clauses = append(clauses, fmt.Sprintf("district = $%d", argCounter))
		args = append(args, *req.District)
		argCounter++
	}

	// If no fields to update, return error
	if len(clauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Build SET clause
	setClauses := ""
	for i, clause := range clauses {
		if i > 0 {
			setClauses += ", "
		}
		setClauses += clause
	}

	query := fmt.Sprintf(`
		UPDATE business_profiles
		SET %s, updated_at = NOW()
		WHERE id = $%d AND deleted_at IS NULL
	`, setClauses, argCounter)

	args = append(args, businessID)

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update business: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("business not found: %s", businessID)
	}

	return nil
}
// GetSellPostStatistics retrieves statistics for SELL type posts
func (r *adminRepository) GetSellPostStatistics(ctx context.Context) (*models.SellStatistics, error) {
	stats := &models.SellStatistics{}

	query := `
		SELECT
			COUNT(*) as total_sell_posts,
			SUM(CASE WHEN sold = true THEN 1 ELSE 0 END) as total_sold,
			SUM(CASE WHEN sold = false AND status = true AND (expired_at IS NULL OR expired_at > NOW()) THEN 1 ELSE 0 END) as total_active,
			SUM(CASE WHEN expired_at IS NOT NULL AND expired_at <= NOW() AND sold = false THEN 1 ELSE 0 END) as total_expired,
			COALESCE(SUM(CASE WHEN sold = true THEN COALESCE(price, 0) ELSE 0 END), 0) as total_revenue,
			COALESCE(AVG(COALESCE(price, 0)), 0) as average_price
		FROM posts
		WHERE type = 'SELL'
			AND deleted_at IS NULL
	`

	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&stats.TotalSellPosts,
		&stats.TotalSold,
		&stats.TotalActive,
		&stats.TotalExpired,
		&stats.TotalRevenue,
		&stats.AveragePrice,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sell post statistics: %w", err)
	}

	return stats, nil
}

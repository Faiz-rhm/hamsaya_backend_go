package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"go.uber.org/zap"
)

// AdminRepository defines the interface for admin operations
type AdminRepository interface {
	GetDashboardStats(ctx context.Context) (*models.DashboardStats, error)
	GetUserAnalytics(ctx context.Context, period string) (*models.UserAnalytics, error)
	GetPostAnalytics(ctx context.Context, period string) (*models.PostAnalytics, error)
	GetEngagementAnalytics(ctx context.Context, period string) (*models.EngagementAnalytics, error)
	
	ListUsers(ctx context.Context, filter *models.AdminUserFilter) ([]*models.AdminUserResponse, int64, error)
	GetUserByID(ctx context.Context, userID string) (*models.AdminUserResponse, error)
	GetUserBio(ctx context.Context, userID string) (*string, error)
	GetUserPosts(ctx context.Context, userID string, limit int) ([]*models.AdminPostResponse, error)
	GetUserBusinesses(ctx context.Context, userID string) ([]*models.AdminBusinessResponse, error)
	SuspendUser(ctx context.Context, userID string, until time.Time) error
	UnsuspendUser(ctx context.Context, userID string) error
	UpdateUserRole(ctx context.Context, userID string, role models.UserRole) error
	DeleteUser(ctx context.Context, userID string) error
	
	ListPosts(ctx context.Context, filter *models.AdminPostFilter) ([]*models.AdminPostResponse, int64, error)
	GetPostByID(ctx context.Context, postID string) (*models.AdminPostDetailResponse, error)
	GetPostComments(ctx context.Context, postID string) ([]models.AdminPostCommentResponse, error)
	UpdatePostStatus(ctx context.Context, postID, status string) error
	DeletePost(ctx context.Context, postID string) error
	
	ListComments(ctx context.Context, filter *models.AdminCommentFilter) ([]*models.AdminCommentResponse, int64, error)
	GetCommentByID(ctx context.Context, commentID string) (*models.AdminCommentDetailResponse, error)
	DeleteComment(ctx context.Context, commentID string) error
	RestoreComment(ctx context.Context, commentID string) error
	ResolveCommentReportsByCommentID(ctx context.Context, commentID string) error
	
	ListBusinesses(ctx context.Context, filter *models.AdminBusinessFilter) ([]*models.AdminBusinessResponse, int64, error)
	GetBusinessByID(ctx context.Context, businessID string) (*models.AdminBusinessDetailResponse, error)
	GetBusinessPosts(ctx context.Context, businessID string, limit int) ([]*models.AdminPostResponse, error)
	GetBusinessHours(ctx context.Context, businessID string) ([]models.AdminBusinessHour, error)
	GetBusinessCategories(ctx context.Context, businessID string) ([]string, error)
	GetBusinessGallery(ctx context.Context, businessID string) ([]models.AttachmentResponse, error)
	UpdateBusinessStatus(ctx context.Context, businessID, status string) error
	DeleteBusiness(ctx context.Context, businessID string) error
	
	ListPostReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminPostReportResponse, int64, error)
	GetPostReportByID(ctx context.Context, reportID string) (*models.AdminPostReportResponse, error)
	ListCommentReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminCommentReportResponse, int64, error)
	GetCommentReportByID(ctx context.Context, reportID string) (*models.AdminCommentReportResponse, error)
	ListUserReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminUserReportResponse, int64, error)
	GetUserReportByID(ctx context.Context, reportID string) (*models.AdminUserReportResponse, error)
	ListBusinessReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminBusinessReportResponse, int64, error)
	GetBusinessReportByID(ctx context.Context, reportID string) (*models.AdminBusinessReportResponse, error)
	UpdatePostReportStatus(ctx context.Context, reportID, status string) error
	UpdateCommentReportStatus(ctx context.Context, reportID, status string) error
	UpdateUserReportResolved(ctx context.Context, reportID string, resolved bool) error
	UpdateBusinessReportStatus(ctx context.Context, reportID, status string) error
	
	GetAllFCMTokens(ctx context.Context) ([]string, error)
	GetFCMTokensByProvince(ctx context.Context, province string) ([]string, error)
	GetFCMTokensByUserIDs(ctx context.Context, userIDs []string) ([]string, error)

	ListFeedback(ctx context.Context, filter *models.AdminFeedbackFilter) ([]*models.AdminFeedbackResponse, int64, error)
}

type adminRepository struct {
	db     *database.DB
	logger *zap.SugaredLogger
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *database.DB) AdminRepository {
	return &adminRepository{
		db:     db,
		logger: utils.GetLogger(),
	}
}

func (r *adminRepository) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	query := `
		SELECT 
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) as total_users,
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE) as new_users_today,
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '7 days') as new_users_week,
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '30 days') as new_users_month,
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND locked_until IS NOT NULL AND locked_until > NOW()) as suspended_users,
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL) as total_posts,
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND type = 'FEED') as total_feed_posts,
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND type = 'EVENT') as total_event_posts,
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND type = 'SELL') as total_sell_posts,
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND type = 'PULL') as total_poll_posts,
			(SELECT COUNT(*) FROM business_profiles WHERE deleted_at IS NULL) as total_businesses,
			(SELECT COUNT(*) FROM business_profiles WHERE deleted_at IS NULL AND status = true) as active_businesses,
			(SELECT COUNT(*) FROM business_profiles WHERE deleted_at IS NULL AND status = false) as pending_businesses,
			(SELECT COUNT(*) FROM business_profiles WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '7 days') as new_businesses_week,
			(SELECT COUNT(*) FROM post_reports WHERE report_status = 'PENDING') + 
			(SELECT COUNT(*) FROM comment_reports WHERE report_status = 'PENDING') + 
			(SELECT COUNT(*) FROM user_reports WHERE resolved = false) + 
			(SELECT COUNT(*) FROM business_reports WHERE report_status = 'PENDING') as pending_reports,
			(SELECT COUNT(*) FROM post_reports WHERE report_status = 'RESOLVED') + 
			(SELECT COUNT(*) FROM comment_reports WHERE report_status = 'RESOLVED') + 
			(SELECT COUNT(*) FROM user_reports WHERE resolved = true) + 
			(SELECT COUNT(*) FROM business_reports WHERE report_status = 'RESOLVED') as resolved_reports,
			(SELECT COUNT(*) FROM post_comments WHERE deleted_at IS NULL) as total_comments,
			(SELECT COUNT(*) FROM post_likes) as total_likes
	`

	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&stats.TotalUsers,
		&stats.NewUsersToday,
		&stats.NewUsersWeek,
		&stats.NewUsersMonth,
		&stats.SuspendedUsers,
		&stats.TotalPosts,
		&stats.TotalFeedPosts,
		&stats.TotalEventPosts,
		&stats.TotalSellPosts,
		&stats.TotalPollPosts,
		&stats.TotalBusinesses,
		&stats.ActiveBusinesses,
		&stats.PendingBusinesses,
		&stats.NewBusinessesWeek,
		&stats.PendingReports,
		&stats.ResolvedReports,
		&stats.TotalComments,
		&stats.TotalLikes,
	)

	if err != nil {
		r.logger.Errorw("Failed to get dashboard stats", "error", err)
		return nil, err
	}

	return stats, nil
}

func (r *adminRepository) GetUserAnalytics(ctx context.Context, period string) (*models.UserAnalytics, error) {
	analytics := &models.UserAnalytics{}
	
	interval := r.getPeriodInterval(period)
	
	growthQuery := fmt.Sprintf(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM users
		WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '%s'
		GROUP BY DATE(created_at)
		ORDER BY date
	`, interval)
	
	rows, err := r.db.Pool.Query(ctx, growthQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var data models.TimeSeriesData
		var date time.Time
		if err := rows.Scan(&date, &data.Count); err != nil {
			return nil, err
		}
		data.Date = date.Format("2006-01-02")
		analytics.GrowthData = append(analytics.GrowthData, data)
	}
	
	countQuery := `
		SELECT 
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) as total_users,
			(SELECT COUNT(DISTINCT user_id) FROM posts WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '30 days') as active_users
	`
	err = r.db.Pool.QueryRow(ctx, countQuery).Scan(&analytics.TotalUsers, &analytics.ActiveUsers)
	if err != nil {
		return nil, err
	}
	
	return analytics, nil
}

func (r *adminRepository) GetPostAnalytics(ctx context.Context, period string) (*models.PostAnalytics, error) {
	analytics := &models.PostAnalytics{}
	
	interval := r.getPeriodInterval(period)
	
	timeQuery := fmt.Sprintf(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM posts
		WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '%s'
		GROUP BY DATE(created_at)
		ORDER BY date
	`, interval)
	
	rows, err := r.db.Pool.Query(ctx, timeQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var data models.TimeSeriesData
		var date time.Time
		if err := rows.Scan(&date, &data.Count); err != nil {
			return nil, err
		}
		data.Date = date.Format("2006-01-02")
		analytics.PostsOverTime = append(analytics.PostsOverTime, data)
	}
	
	typeQuery := `
		SELECT type, COUNT(*) as count
		FROM posts
		WHERE deleted_at IS NULL
		GROUP BY type
	`
	
	typeRows, err := r.db.Pool.Query(ctx, typeQuery)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()
	
	for typeRows.Next() {
		var data models.PostTypeCount
		if err := typeRows.Scan(&data.Type, &data.Count); err != nil {
			return nil, err
		}
		analytics.PostsByType = append(analytics.PostsByType, data)
	}
	
	err = r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL").Scan(&analytics.TotalPosts)
	if err != nil {
		return nil, err
	}
	
	return analytics, nil
}

func (r *adminRepository) GetEngagementAnalytics(ctx context.Context, period string) (*models.EngagementAnalytics, error) {
	analytics := &models.EngagementAnalytics{}
	
	interval := r.getPeriodInterval(period)
	
	likesQuery := fmt.Sprintf(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM post_likes
		WHERE created_at >= CURRENT_DATE - INTERVAL '%s'
		GROUP BY DATE(created_at)
		ORDER BY date
	`, interval)
	
	rows, err := r.db.Pool.Query(ctx, likesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var data models.TimeSeriesData
		var date time.Time
		if err := rows.Scan(&date, &data.Count); err != nil {
			return nil, err
		}
		data.Date = date.Format("2006-01-02")
		analytics.LikesOverTime = append(analytics.LikesOverTime, data)
	}
	
	commentsQuery := fmt.Sprintf(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM post_comments
		WHERE deleted_at IS NULL AND created_at >= CURRENT_DATE - INTERVAL '%s'
		GROUP BY DATE(created_at)
		ORDER BY date
	`, interval)
	
	commentRows, err := r.db.Pool.Query(ctx, commentsQuery)
	if err != nil {
		return nil, err
	}
	defer commentRows.Close()
	
	for commentRows.Next() {
		var data models.TimeSeriesData
		var date time.Time
		if err := commentRows.Scan(&date, &data.Count); err != nil {
			return nil, err
		}
		data.Date = date.Format("2006-01-02")
		analytics.CommentsOverTime = append(analytics.CommentsOverTime, data)
	}
	
	totalsQuery := `
		SELECT 
			(SELECT COUNT(*) FROM post_likes) as total_likes,
			(SELECT COUNT(*) FROM post_comments WHERE deleted_at IS NULL) as total_comments,
			(SELECT COUNT(*) FROM post_shares) as total_shares
	`
	err = r.db.Pool.QueryRow(ctx, totalsQuery).Scan(&analytics.TotalLikes, &analytics.TotalComments, &analytics.TotalShares)
	if err != nil {
		return nil, err
	}
	
	return analytics, nil
}

func (r *adminRepository) ListUsers(ctx context.Context, filter *models.AdminUserFilter) ([]*models.AdminUserResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1
	
	conditions = append(conditions, "u.deleted_at IS NULL")
	
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(u.email ILIKE $%d OR p.first_name ILIKE $%d OR p.last_name ILIKE $%d)", argIndex, argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}
	
	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("u.role = $%d", argIndex))
		args = append(args, filter.Role)
		argIndex++
	}
	
	if filter.Status == "suspended" {
		conditions = append(conditions, "u.locked_until > NOW()")
	} else if filter.Status == "active" {
		conditions = append(conditions, "(u.locked_until IS NULL OR u.locked_until <= NOW())")
	}
	
	if filter.Province != "" {
		conditions = append(conditions, fmt.Sprintf("p.province = $%d", argIndex))
		args = append(args, filter.Province)
		argIndex++
	}
	
	whereClause := strings.Join(conditions, " AND ")
	
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN profiles p ON u.id = p.id
		WHERE %s
	`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	sortBy := "u.created_at"
	if filter.SortBy == "email" {
		sortBy = "u.email"
	} else if filter.SortBy == "name" {
		sortBy = "p.first_name"
	}
	
	sortDir := "DESC"
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			u.id, u.email, u.phone, u.email_verified, u.mfa_enabled, u.role,
			p.first_name, p.last_name, p.avatar, p.cover, p.country, p.province, p.district, p.neighborhood, p.is_complete,
			u.locked_until, u.last_login_at, u.created_at,
			(SELECT COUNT(*) FROM posts WHERE user_id = u.id AND deleted_at IS NULL) as posts_count,
			(SELECT COUNT(*) FROM user_follows WHERE following_id = u.id) as followers_count,
			(SELECT COUNT(*) FROM user_follows WHERE follower_id = u.id) as following_count
		FROM users u
		LEFT JOIN profiles p ON u.id = p.id
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortBy, sortDir, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var users []*models.AdminUserResponse
	for rows.Next() {
		user := &models.AdminUserResponse{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.Phone, &user.EmailVerified, &user.MFAEnabled, &user.Role,
			&user.FirstName, &user.LastName, &user.Avatar, &user.Cover,
			&user.Country, &user.Province, &user.District, &user.Neighborhood,
			&user.IsComplete,
			&user.LockedUntil, &user.LastLoginAt, &user.CreatedAt,
			&user.PostsCount, &user.FollowersCount, &user.FollowingCount,
		)
		if err != nil {
			return nil, 0, err
		}
		user.IsSuspended = user.LockedUntil != nil && user.LockedUntil.After(time.Now())
		users = append(users, user)
	}
	
	return users, totalCount, nil
}

func (r *adminRepository) GetUserByID(ctx context.Context, userID string) (*models.AdminUserResponse, error) {
	query := `
		SELECT 
			u.id, u.email, u.phone, u.email_verified, u.mfa_enabled, u.role,
			p.first_name, p.last_name, p.avatar, p.cover, p.country, p.province, p.district, p.neighborhood, p.is_complete,
			u.locked_until, u.last_login_at, u.created_at,
			(SELECT COUNT(*) FROM posts WHERE user_id = u.id AND deleted_at IS NULL) as posts_count,
			(SELECT COUNT(*) FROM user_follows WHERE following_id = u.id) as followers_count,
			(SELECT COUNT(*) FROM user_follows WHERE follower_id = u.id) as following_count
		FROM users u
		LEFT JOIN profiles p ON u.id = p.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`
	
	user := &models.AdminUserResponse{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Phone, &user.EmailVerified, &user.MFAEnabled, &user.Role,
		&user.FirstName, &user.LastName, &user.Avatar, &user.Cover,
		&user.Country, &user.Province, &user.District, &user.Neighborhood,
		&user.IsComplete,
		&user.LockedUntil, &user.LastLoginAt, &user.CreatedAt,
		&user.PostsCount, &user.FollowersCount, &user.FollowingCount,
	)
	if err != nil {
		return nil, err
	}
	
	user.IsSuspended = user.LockedUntil != nil && user.LockedUntil.After(time.Now())
	return user, nil
}

func (r *adminRepository) GetUserBio(ctx context.Context, userID string) (*string, error) {
	query := `SELECT bio FROM profiles WHERE id = $1`
	var bio *string
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&bio)
	if err != nil {
		return nil, nil // bio not found is okay
	}
	return bio, nil
}

func (r *adminRepository) GetUserPosts(ctx context.Context, userID string, limit int) ([]*models.AdminPostResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			p.id, p.type, p.title, p.description,
			CASE WHEN p.status = true THEN 'ACTIVE' ELSE 'HIDDEN' END as status,
			p.user_id::text as author_id,
			u.email as author_email,
			COALESCE(NULLIF(trim(COALESCE(pr.first_name,'') || ' ' || COALESCE(pr.last_name,'')), ''), u.email) as author_name,
			p.total_likes, p.total_comments, p.total_shares,
			(SELECT COUNT(*) FROM post_reports WHERE post_id = p.id) as report_count,
			p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE p.deleted_at IS NULL AND p.user_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.AdminPostResponse
	for rows.Next() {
		post := &models.AdminPostResponse{}
		err := rows.Scan(
			&post.ID, &post.Type, &post.Title, &post.Description, &post.Status,
			&post.AuthorID, &post.AuthorEmail, &post.AuthorName,
			&post.TotalLikes, &post.TotalComments, &post.TotalShares,
			&post.ReportCount,
			&post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	if posts == nil {
		posts = []*models.AdminPostResponse{}
	}
	return posts, nil
}

func (r *adminRepository) GetUserBusinesses(ctx context.Context, userID string) ([]*models.AdminBusinessResponse, error) {
	query := `
		SELECT 
			bp.id, bp.name, bp.description,
			CASE WHEN bp.status = true THEN 'ACTIVE' ELSE 'INACTIVE' END as status,
			bp.user_id, u.email,
			COALESCE(NULLIF(trim(COALESCE(pr.first_name,'') || ' ' || COALESCE(pr.last_name,'')), ''), u.email) as owner_name,
			bp.avatar, bp.province,
			bp.total_follow, bp.total_views,
			(SELECT COUNT(*) FROM business_reports WHERE business_id = bp.id) as report_count,
			bp.created_at
		FROM business_profiles bp
		JOIN users u ON bp.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE bp.user_id = $1 AND bp.deleted_at IS NULL
		ORDER BY bp.created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var businesses []*models.AdminBusinessResponse
	for rows.Next() {
		b := &models.AdminBusinessResponse{}
		err := rows.Scan(
			&b.ID, &b.Name, &b.Description, &b.Status,
			&b.OwnerID, &b.OwnerEmail, &b.OwnerName,
			&b.Avatar, &b.Province,
			&b.TotalFollow, &b.TotalViews,
			&b.ReportCount,
			&b.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		businesses = append(businesses, b)
	}

	if businesses == nil {
		businesses = []*models.AdminBusinessResponse{}
	}
	return businesses, nil
}

func (r *adminRepository) SuspendUser(ctx context.Context, userID string, until time.Time) error {
	query := `UPDATE users SET locked_until = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, until, userID)
	return err
}

func (r *adminRepository) UnsuspendUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET locked_until = NULL, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

func (r *adminRepository) UpdateUserRole(ctx context.Context, userID string, role models.UserRole) error {
	query := `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, role, userID)
	return err
}

func (r *adminRepository) DeleteUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	return err
}

func (r *adminRepository) ListPosts(ctx context.Context, filter *models.AdminPostFilter) ([]*models.AdminPostResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1
	
	conditions = append(conditions, "p.deleted_at IS NULL")
	
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(p.title ILIKE $%d OR p.description ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}
	
	if filter.Type != "" && filter.Type != "all" {
		conditions = append(conditions, fmt.Sprintf("p.type = $%d", argIndex))
		args = append(args, filter.Type)
		argIndex++
	}
	
	if filter.Status != "" {
		// posts.status is boolean: true = visible, false = hidden
		statusBool := filter.Status == "ACTIVE" || filter.Status == "true"
		conditions = append(conditions, fmt.Sprintf("p.status = $%d", argIndex))
		args = append(args, statusBool)
		argIndex++
	}
	
	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("p.user_id = $%d", argIndex))
		args = append(args, filter.UserID)
		argIndex++
	}
	
	if filter.Reported {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM post_reports pr WHERE pr.post_id = p.id)")
	}
	
	whereClause := strings.Join(conditions, " AND ")
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM posts p WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	sortBy := "p.created_at"
	sortDir := "DESC"
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			p.id, p.type, p.title, p.description,
			CASE WHEN p.status = true THEN 'ACTIVE' ELSE 'HIDDEN' END as status,
			COALESCE(p.user_id::text, p.business_id::text, '') as author_id,
			COALESCE(u.email, '') as author_email,
			COALESCE(NULLIF(trim(pr.first_name || ' ' || pr.last_name), ''), u.email, bp.name, '—') as author_name,
			p.total_likes, p.total_comments, p.total_shares,
			(SELECT COUNT(*) FROM post_reports WHERE post_id = p.id) as report_count,
			p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		LEFT JOIN business_profiles bp ON p.business_id = bp.id
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortBy, sortDir, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var posts []*models.AdminPostResponse
	for rows.Next() {
		post := &models.AdminPostResponse{}
		err := rows.Scan(
			&post.ID, &post.Type, &post.Title, &post.Description, &post.Status,
			&post.AuthorID, &post.AuthorEmail, &post.AuthorName,
			&post.TotalLikes, &post.TotalComments, &post.TotalShares,
			&post.ReportCount,
			&post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		posts = append(posts, post)
	}
	
	return posts, totalCount, nil
}

func (r *adminRepository) GetPostByID(ctx context.Context, postID string) (*models.AdminPostDetailResponse, error) {
	query := `
		SELECT 
			p.id, p.type, p.title, p.description,
			CASE WHEN p.status = true THEN 'ACTIVE' ELSE 'HIDDEN' END as status,
			COALESCE(p.visibility::text, 'PUBLIC') as visibility,
			COALESCE(p.user_id::text, p.business_id::text, '') as author_id,
			COALESCE(u.email, '') as author_email,
			COALESCE(NULLIF(trim(COALESCE(pr.first_name,'') || ' ' || COALESCE(pr.last_name,'')), ''), u.email, bp.name, '—') as author_name,
			pr.avatar,
			p.business_id, bp.name as business_name,
			p.category_id,
			(SELECT sc.name FROM sell_categories sc WHERE sc.id = p.category_id) as category_name,
			p.currency, p.price, p.discount, p.free, p.sold, p.is_promoted, p.contact_no,
			p.start_date, p.end_date, p.event_state::text,
			p.interested_count, p.going_count,
			p.country, p.province, p.district, p.neighborhood,
			p.total_likes, p.total_comments, p.total_shares,
			(SELECT COUNT(*) FROM post_reports WHERE post_id = p.id) as report_count,
			p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		LEFT JOIN business_profiles bp ON p.business_id = bp.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`

	post := &models.AdminPostDetailResponse{}
	var authorAvatar *models.Photo
	var eventState *string
	err := r.db.Pool.QueryRow(ctx, query, postID).Scan(
		&post.ID, &post.Type, &post.Title, &post.Description, &post.Status,
		&post.Visibility,
		&post.AuthorID, &post.AuthorEmail, &post.AuthorName,
		&authorAvatar,
		&post.BusinessID, &post.BusinessName,
		&post.CategoryID, &post.CategoryName,
		&post.Currency, &post.Price, &post.Discount, &post.Free, &post.Sold, &post.IsPromoted, &post.ContactNo,
		&post.StartDate, &post.EndDate, &eventState,
		&post.InterestedCount, &post.GoingCount,
		&post.Country, &post.Province, &post.District, &post.Neighborhood,
		&post.TotalLikes, &post.TotalComments, &post.TotalShares,
		&post.ReportCount,
		&post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	post.AuthorAvatar = authorAvatar
	post.EventState = eventState

	// Fetch attachments
	attachRows, err := r.db.Pool.Query(ctx, `
		SELECT id, photo FROM attachments
		WHERE post_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`, postID)
	if err != nil {
		return nil, err
	}
	defer attachRows.Close()

	post.Attachments = []models.AttachmentResponse{}
	for attachRows.Next() {
		var a models.AttachmentResponse
		if err := attachRows.Scan(&a.ID, &a.Photo); err != nil {
			return nil, err
		}
		post.Attachments = append(post.Attachments, a)
	}

	return post, nil
}

func (r *adminRepository) GetPostComments(ctx context.Context, postID string) ([]models.AdminPostCommentResponse, error) {
	query := `
		SELECT
			c.id, c.text,
			c.user_id, u.email,
			COALESCE(NULLIF(trim(COALESCE(pr.first_name,'') || ' ' || COALESCE(pr.last_name,'')), ''), u.email) as author_name,
			pr.avatar,
			c.parent_comment_id,
			c.total_likes, c.total_replies,
			(SELECT COUNT(*) FROM comment_reports WHERE comment_id = c.id) as report_count,
			c.created_at
		FROM post_comments c
		JOIN users u ON c.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE c.post_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.created_at ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type commentRow struct {
		models.AdminPostCommentResponse
		ParentID *string
	}
	var allComments []commentRow

	for rows.Next() {
		var cr commentRow
		var avatar *models.Photo
		var parentID *string
		err := rows.Scan(
			&cr.ID, &cr.Text,
			&cr.AuthorID, &cr.AuthorEmail, &cr.AuthorName,
			&avatar,
			&parentID,
			&cr.TotalLikes, &cr.TotalReplies,
			&cr.ReportCount,
			&cr.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		cr.AuthorAvatar = avatar
		cr.ParentID = parentID
		cr.Replies = []models.AdminPostCommentResponse{}
		allComments = append(allComments, cr)
	}

	// Build tree: top-level comments + nested replies
	commentMap := make(map[string]*models.AdminPostCommentResponse)
	var topLevel []models.AdminPostCommentResponse

	for i := range allComments {
		c := allComments[i].AdminPostCommentResponse
		commentMap[c.ID] = &c
	}

	for i := range allComments {
		c := allComments[i]
		if c.ParentID != nil {
			if parent, ok := commentMap[*c.ParentID]; ok {
				parent.Replies = append(parent.Replies, c.AdminPostCommentResponse)
			}
		}
	}

	for i := range allComments {
		if allComments[i].ParentID == nil {
			if built, ok := commentMap[allComments[i].ID]; ok {
				topLevel = append(topLevel, *built)
			}
		}
	}

	if topLevel == nil {
		topLevel = []models.AdminPostCommentResponse{}
	}
	return topLevel, nil
}

func (r *adminRepository) UpdatePostStatus(ctx context.Context, postID, status string) error {
	// posts.status is boolean: ACTIVE = true, HIDDEN = false
	statusBool := status == "ACTIVE" || status == "true"
	query := `UPDATE posts SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, statusBool, postID)
	return err
}

func (r *adminRepository) DeletePost(ctx context.Context, postID string) error {
	query := `UPDATE posts SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, postID)
	return err
}

func (r *adminRepository) GetCommentByID(ctx context.Context, commentID string) (*models.AdminCommentDetailResponse, error) {
	query := `
		SELECT 
			c.id, c.text, c.post_id, p.title,
			c.user_id, u.email, COALESCE(NULLIF(trim(pr.first_name || ' ' || pr.last_name), ''), u.email) as author_name,
			c.total_likes,
			(SELECT COUNT(*) FROM comment_reports WHERE comment_id = c.id) as report_count,
			c.created_at, c.deleted_at
		FROM post_comments c
		LEFT JOIN posts p ON c.post_id = p.id
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE c.id = $1
	`
	out := &models.AdminCommentDetailResponse{}
	var postTitle *string
	err := r.db.Pool.QueryRow(ctx, query, commentID).Scan(
		&out.ID, &out.Content, &out.PostID, &postTitle,
		&out.AuthorID, &out.AuthorEmail, &out.AuthorName,
		&out.TotalLikes, &out.ReportCount,
		&out.CreatedAt, &out.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	out.PostTitle = postTitle
	return out, nil
}

func (r *adminRepository) ListComments(ctx context.Context, filter *models.AdminCommentFilter) ([]*models.AdminCommentResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	conditions = append(conditions, "c.deleted_at IS NULL")

	if filter.CommentID != "" {
		conditions = append(conditions, fmt.Sprintf("c.id = $%d", argIndex))
		args = append(args, filter.CommentID)
		argIndex++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("c.text ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}

	if filter.PostID != "" {
		conditions = append(conditions, fmt.Sprintf("c.post_id = $%d", argIndex))
		args = append(args, filter.PostID)
		argIndex++
	}
	
	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("c.user_id = $%d", argIndex))
		args = append(args, filter.UserID)
		argIndex++
	}
	
	if filter.Reported {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM comment_reports cr WHERE cr.comment_id = c.id)")
	}
	
	whereClause := strings.Join(conditions, " AND ")
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM post_comments c WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			c.id, c.text, c.post_id, p.title,
			c.user_id, u.email, COALESCE(NULLIF(trim(pr.first_name || ' ' || pr.last_name), ''), u.email) as author_name,
			c.total_likes,
			(SELECT COUNT(*) FROM comment_reports WHERE comment_id = c.id) as report_count,
			c.created_at
		FROM post_comments c
		JOIN posts p ON c.post_id = p.id
		JOIN users u ON c.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE %s
		ORDER BY c.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var comments []*models.AdminCommentResponse
	for rows.Next() {
		comment := &models.AdminCommentResponse{}
		err := rows.Scan(
			&comment.ID, &comment.Content, &comment.PostID, &comment.PostTitle,
			&comment.AuthorID, &comment.AuthorEmail, &comment.AuthorName,
			&comment.TotalLikes, &comment.ReportCount,
			&comment.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		comments = append(comments, comment)
	}
	
	return comments, totalCount, nil
}

func (r *adminRepository) DeleteComment(ctx context.Context, commentID string) error {
	query := `UPDATE post_comments SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, commentID)
	return err
}

// RestoreComment clears deleted_at for a soft-deleted comment (unhide)
func (r *adminRepository) RestoreComment(ctx context.Context, commentID string) error {
	query := `UPDATE post_comments SET deleted_at = NULL, updated_at = NOW() WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, commentID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("comment not found or not deleted")
	}
	return nil
}

// ResolveCommentReportsByCommentID sets report_status = 'RESOLVED' for all reports targeting this comment
func (r *adminRepository) ResolveCommentReportsByCommentID(ctx context.Context, commentID string) error {
	query := `UPDATE comment_reports SET report_status = 'RESOLVED', updated_at = NOW() WHERE comment_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, commentID)
	return err
}

func (r *adminRepository) ListBusinesses(ctx context.Context, filter *models.AdminBusinessFilter) ([]*models.AdminBusinessResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1
	
	conditions = append(conditions, "b.deleted_at IS NULL")
	
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(b.name ILIKE $%d OR b.description ILIKE $%d)", argIndex, argIndex))
		args = append(args, "%"+filter.Search+"%")
		argIndex++
	}
	
	if filter.Status != "" {
		// Convert string status to boolean (status column is boolean in DB)
		// ACTIVE = true, anything else (PENDING, SUSPENDED, REJECTED) = false
		statusBool := filter.Status == "ACTIVE"
		conditions = append(conditions, fmt.Sprintf("b.status = $%d", argIndex))
		args = append(args, statusBool)
		argIndex++
	}

	if filter.Province != "" {
		conditions = append(conditions, fmt.Sprintf("b.province ILIKE $%d", argIndex))
		args = append(args, "%"+filter.Province+"%")
		argIndex++
	}

	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM business_categories bc
				JOIN categories c ON bc.category_id = c.id
				WHERE bc.business_id = b.id AND c.name ILIKE $%d
			)
		`, argIndex))
		args = append(args, "%"+filter.Category+"%")
		argIndex++
	}

	whereClause := strings.Join(conditions, " AND ")
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM business_profiles b WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			b.id, b.name, b.description, 
			CASE WHEN b.status = true THEN 'ACTIVE' ELSE 'INACTIVE' END as status,
			b.user_id, u.email, COALESCE(pr.first_name || ' ' || pr.last_name, u.email) as owner_name,
			b.avatar, b.province, b.total_follow, b.total_views,
			(SELECT COUNT(*) FROM business_reports WHERE business_id = b.id) as report_count,
			b.created_at
		FROM business_profiles b
		JOIN users u ON b.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE %s
		ORDER BY b.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var businesses []*models.AdminBusinessResponse
	for rows.Next() {
		business := &models.AdminBusinessResponse{}
		err := rows.Scan(
			&business.ID, &business.Name, &business.Description, &business.Status,
			&business.OwnerID, &business.OwnerEmail, &business.OwnerName,
			&business.Avatar, &business.Province, &business.TotalFollow, &business.TotalViews,
			&business.ReportCount,
			&business.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		businesses = append(businesses, business)
	}
	
	return businesses, totalCount, nil
}

func (r *adminRepository) GetBusinessByID(ctx context.Context, businessID string) (*models.AdminBusinessDetailResponse, error) {
	query := `
		SELECT 
			b.id, b.name, b.license_no, b.description, b.address,
			b.phone_number, b.email, b.website,
			b.avatar, b.avatar_color, b.cover,
			CASE WHEN b.status = true THEN 'ACTIVE' ELSE 'INACTIVE' END as status,
			b.additional_info,
			b.country, b.province, b.district, b.neighborhood, b.show_location,
			b.user_id, u.email,
			COALESCE(NULLIF(trim(COALESCE(pr.first_name,'') || ' ' || COALESCE(pr.last_name,'')), ''), u.email) as owner_name,
			pr.avatar as owner_avatar,
			b.total_follow, b.total_views,
			(SELECT COUNT(*) FROM posts WHERE business_id = b.id AND deleted_at IS NULL) as total_posts,
			(SELECT COUNT(*) FROM business_reports WHERE business_id = b.id) as report_count,
			b.created_at, b.updated_at
		FROM business_profiles b
		JOIN users u ON b.user_id = u.id
		LEFT JOIN profiles pr ON u.id = pr.id
		WHERE b.id = $1 AND b.deleted_at IS NULL
	`

	business := &models.AdminBusinessDetailResponse{}
	err := r.db.Pool.QueryRow(ctx, query, businessID).Scan(
		&business.ID, &business.Name, &business.LicenseNo, &business.Description, &business.Address,
		&business.PhoneNumber, &business.Email, &business.Website,
		&business.Avatar, &business.AvatarColor, &business.Cover,
		&business.Status,
		&business.AdditionalInfo,
		&business.Country, &business.Province, &business.District, &business.Neighborhood, &business.ShowLocation,
		&business.OwnerID, &business.OwnerEmail, &business.OwnerName,
		&business.OwnerAvatar,
		&business.TotalFollow, &business.TotalViews, &business.TotalPosts,
		&business.ReportCount,
		&business.CreatedAt, &business.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return business, nil
}

func (r *adminRepository) GetBusinessPosts(ctx context.Context, businessID string, limit int) ([]*models.AdminPostResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			p.id, p.type, p.title, p.description,
			CASE WHEN p.status = true THEN 'ACTIVE' ELSE 'HIDDEN' END as status,
			COALESCE(p.business_id::text, '') as author_id,
			'' as author_email,
			COALESCE(bp.name, '—') as author_name,
			p.total_likes, p.total_comments, p.total_shares,
			(SELECT COUNT(*) FROM post_reports WHERE post_id = p.id) as report_count,
			p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN business_profiles bp ON p.business_id = bp.id
		WHERE p.business_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*models.AdminPostResponse
	for rows.Next() {
		post := &models.AdminPostResponse{}
		err := rows.Scan(
			&post.ID, &post.Type, &post.Title, &post.Description, &post.Status,
			&post.AuthorID, &post.AuthorEmail, &post.AuthorName,
			&post.TotalLikes, &post.TotalComments, &post.TotalShares,
			&post.ReportCount,
			&post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	if posts == nil {
		posts = []*models.AdminPostResponse{}
	}
	return posts, nil
}

func (r *adminRepository) GetBusinessHours(ctx context.Context, businessID string) ([]models.AdminBusinessHour, error) {
	query := `
		SELECT day, open_time::text, close_time::text, is_closed
		FROM business_hours
		WHERE business_profile_id = $1
		ORDER BY 
			CASE day 
				WHEN 'MONDAY' THEN 1
				WHEN 'TUESDAY' THEN 2
				WHEN 'WEDNESDAY' THEN 3
				WHEN 'THURSDAY' THEN 4
				WHEN 'FRIDAY' THEN 5
				WHEN 'SATURDAY' THEN 6
				WHEN 'SUNDAY' THEN 7
			END
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return []models.AdminBusinessHour{}, nil
	}
	defer rows.Close()

	var hours []models.AdminBusinessHour
	for rows.Next() {
		var h models.AdminBusinessHour
		if err := rows.Scan(&h.Day, &h.OpenTime, &h.CloseTime, &h.IsClosed); err != nil {
			continue
		}
		hours = append(hours, h)
	}

	if hours == nil {
		hours = []models.AdminBusinessHour{}
	}
	return hours, nil
}

func (r *adminRepository) GetBusinessCategories(ctx context.Context, businessID string) ([]string, error) {
	query := `
		SELECT bc.name
		FROM business_profile_categories bpc
		JOIN business_categories bc ON bpc.business_category_id = bc.id
		WHERE bpc.business_profile_id = $1
		ORDER BY bc.name
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return []string{}, nil
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		categories = append(categories, name)
	}

	if categories == nil {
		categories = []string{}
	}
	return categories, nil
}

func (r *adminRepository) GetBusinessGallery(ctx context.Context, businessID string) ([]models.AttachmentResponse, error) {
	query := `
		SELECT id, photo
		FROM business_attachments
		WHERE business_profile_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`

	rows, err := r.db.Pool.Query(ctx, query, businessID)
	if err != nil {
		return []models.AttachmentResponse{}, nil
	}
	defer rows.Close()

	var gallery []models.AttachmentResponse
	for rows.Next() {
		var a models.AttachmentResponse
		if err := rows.Scan(&a.ID, &a.Photo); err != nil {
			continue
		}
		gallery = append(gallery, a)
	}

	if gallery == nil {
		gallery = []models.AttachmentResponse{}
	}
	return gallery, nil
}

func (r *adminRepository) UpdateBusinessStatus(ctx context.Context, businessID, status string) error {
	// Convert string status to boolean (status column is boolean in DB)
	// ACTIVE = true, anything else (PENDING, SUSPENDED, REJECTED) = false
	statusBool := status == "ACTIVE"
	query := `UPDATE business_profiles SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, statusBool, businessID)
	return err
}

func (r *adminRepository) DeleteBusiness(ctx context.Context, businessID string) error {
	query := `UPDATE business_profiles SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, businessID)
	return err
}

func (r *adminRepository) ListPostReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminPostReportResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.PostID != "" {
		conditions = append(conditions, fmt.Sprintf("r.post_id = $%d", argIndex))
		args = append(args, filter.PostID)
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("r.report_status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}
	
	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM post_reports r WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			r.id, r.post_id, p.title,
			p.user_id, pu.email,
			r.user_id, ru.email,
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM post_reports r
		JOIN posts p ON r.post_id = p.id
		JOIN users pu ON p.user_id = pu.id
		JOIN users ru ON r.user_id = ru.id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	reports := []*models.AdminPostReportResponse{}
	for rows.Next() {
		report := &models.AdminPostReportResponse{}
		err := rows.Scan(
			&report.ID, &report.PostID, &report.PostTitle,
			&report.PostAuthorID, &report.PostAuthorEmail,
			&report.ReporterID, &report.ReporterEmail,
			&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}
	
	return reports, totalCount, nil
}

func (r *adminRepository) GetPostReportByID(ctx context.Context, reportID string) (*models.AdminPostReportResponse, error) {
	// Use LEFT JOINs so we return the report even if the post or a user was deleted
	query := `
		SELECT 
			r.id, r.post_id,
			p.title,
			CASE 
				WHEN p.deleted_at IS NOT NULL THEN 'DELETED'
				WHEN p.status = false THEN 'HIDDEN'
				ELSE 'ACTIVE'
			END,
			COALESCE(p.user_id::text, ''),
			COALESCE(pu.email, ''),
			r.user_id::text,
			COALESCE(ru.email, ''),
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM post_reports r
		LEFT JOIN posts p ON r.post_id = p.id
		LEFT JOIN users pu ON p.user_id = pu.id
		LEFT JOIN users ru ON r.user_id = ru.id
		WHERE r.id = $1
	`
	report := &models.AdminPostReportResponse{}
	var postTitle *string
	err := r.db.Pool.QueryRow(ctx, query, reportID).Scan(
		&report.ID, &report.PostID,
		&postTitle, &report.PostStatus,
		&report.PostAuthorID, &report.PostAuthorEmail,
		&report.ReporterID, &report.ReporterEmail,
		&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	report.PostTitle = postTitle
	return report, nil
}

func (r *adminRepository) ListCommentReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminCommentReportResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.CommentID != "" {
		conditions = append(conditions, fmt.Sprintf("r.comment_id = $%d", argIndex))
		args = append(args, filter.CommentID)
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("r.report_status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM comment_reports r WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			r.id, r.comment_id,
			COALESCE(c.post_id::text, ''),
			COALESCE(c.text, ''),
			COALESCE(c.user_id::text, ''),
			COALESCE(cu.email, ''),
			r.user_id::text,
			COALESCE(ru.email, ''),
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM comment_reports r
		LEFT JOIN post_comments c ON r.comment_id = c.id
		LEFT JOIN users cu ON c.user_id = cu.id
		LEFT JOIN users ru ON r.user_id = ru.id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	reports := []*models.AdminCommentReportResponse{}
	for rows.Next() {
		report := &models.AdminCommentReportResponse{}
		err := rows.Scan(
			&report.ID, &report.CommentID, &report.PostID, &report.CommentContent,
			&report.CommentAuthorID, &report.CommentAuthorEmail,
			&report.ReporterID, &report.ReporterEmail,
			&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}
	
	return reports, totalCount, nil
}

func (r *adminRepository) GetCommentReportByID(ctx context.Context, reportID string) (*models.AdminCommentReportResponse, error) {
	query := `
		SELECT 
			r.id, r.comment_id,
			COALESCE(c.post_id::text, ''),
			COALESCE(c.text, ''),
			COALESCE(c.user_id::text, ''),
			COALESCE(cu.email, ''),
			COALESCE((c.deleted_at IS NOT NULL), false),
			r.user_id::text,
			COALESCE(ru.email, ''),
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM comment_reports r
		LEFT JOIN post_comments c ON r.comment_id = c.id
		LEFT JOIN users cu ON c.user_id = cu.id
		LEFT JOIN users ru ON r.user_id = ru.id
		WHERE r.id = $1
	`
	report := &models.AdminCommentReportResponse{}
	err := r.db.Pool.QueryRow(ctx, query, reportID).Scan(
		&report.ID, &report.CommentID, &report.PostID, &report.CommentContent,
		&report.CommentAuthorID, &report.CommentAuthorEmail, &report.CommentHidden,
		&report.ReporterID, &report.ReporterEmail,
		&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (r *adminRepository) ListUserReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminUserReportResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("r.reported_user = $%d", argIndex))
		args = append(args, filter.UserID)
		argIndex++
	}

	if filter.Status == "RESOLVED" {
		conditions = append(conditions, "r.resolved = true")
	} else if filter.Status == "PENDING" {
		conditions = append(conditions, "r.resolved = false")
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM user_reports r WHERE %s`, whereClause)

	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}

	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit

	query := fmt.Sprintf(`
		SELECT 
			r.id, r.reported_user::text,
			COALESCE(ru.email, ''),
			COALESCE(rp.first_name || ' ' || rp.last_name, COALESCE(ru.email, '')),
			r.reported_by_id::text,
			COALESCE(rb.email, ''),
			r.reason, r.description, r.resolved, r.created_at
		FROM user_reports r
		LEFT JOIN users ru ON r.reported_user = ru.id
		LEFT JOIN profiles rp ON ru.id = rp.id
		LEFT JOIN users rb ON r.reported_by_id = rb.id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	reports := []*models.AdminUserReportResponse{}
	for rows.Next() {
		report := &models.AdminUserReportResponse{}
		err := rows.Scan(
			&report.ID, &report.ReportedUserID, &report.ReportedUserEmail, &report.ReportedUserName,
			&report.ReporterID, &report.ReporterEmail,
			&report.Reason, &report.Description, &report.Resolved, &report.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}
	
	return reports, totalCount, nil
}

func (r *adminRepository) GetUserReportByID(ctx context.Context, reportID string) (*models.AdminUserReportResponse, error) {
	query := `
		SELECT 
			r.id, r.reported_user::text,
			COALESCE(ru.email, ''),
			COALESCE(rp.first_name || ' ' || rp.last_name, COALESCE(ru.email, '')),
			COALESCE((ru.locked_until IS NOT NULL AND ru.locked_until > NOW()), false),
			r.reported_by_id::text,
			COALESCE(rb.email, ''),
			r.reason, r.description, r.resolved, r.created_at
		FROM user_reports r
		LEFT JOIN users ru ON r.reported_user = ru.id
		LEFT JOIN profiles rp ON ru.id = rp.id
		LEFT JOIN users rb ON r.reported_by_id = rb.id
		WHERE r.id = $1
	`
	report := &models.AdminUserReportResponse{}
	err := r.db.Pool.QueryRow(ctx, query, reportID).Scan(
		&report.ID, &report.ReportedUserID, &report.ReportedUserEmail, &report.ReportedUserName,
		&report.ReportedUserSuspended,
		&report.ReporterID, &report.ReporterEmail,
		&report.Reason, &report.Description, &report.Resolved, &report.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (r *adminRepository) ListBusinessReports(ctx context.Context, filter *models.AdminReportFilter) ([]*models.AdminBusinessReportResponse, int64, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.BusinessID != "" {
		conditions = append(conditions, fmt.Sprintf("r.business_id = $%d", argIndex))
		args = append(args, filter.BusinessID)
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("r.report_status = $%d", argIndex))
		args = append(args, filter.Status)
		argIndex++
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}
	
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM business_reports r WHERE %s`, whereClause)
	
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit
	
	query := fmt.Sprintf(`
		SELECT 
			r.id, r.business_id::text,
			COALESCE(b.name, ''),
			COALESCE(b.user_id::text, ''),
			COALESCE(bu.email, ''),
			r.user_id::text,
			COALESCE(ru.email, ''),
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM business_reports r
		LEFT JOIN business_profiles b ON r.business_id = b.id
		LEFT JOIN users bu ON b.user_id = bu.id
		LEFT JOIN users ru ON r.user_id = ru.id
		WHERE %s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	
	args = append(args, limit, offset)
	
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	reports := []*models.AdminBusinessReportResponse{}
	for rows.Next() {
		report := &models.AdminBusinessReportResponse{}
		err := rows.Scan(
			&report.ID, &report.BusinessID, &report.BusinessName,
			&report.BusinessOwnerID, &report.BusinessOwnerEmail,
			&report.ReporterID, &report.ReporterEmail,
			&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}
	
	return reports, totalCount, nil
}

func (r *adminRepository) GetBusinessReportByID(ctx context.Context, reportID string) (*models.AdminBusinessReportResponse, error) {
	query := `
		SELECT 
			r.id, r.business_id::text,
			COALESCE(b.name, ''),
			CASE 
				WHEN b.deleted_at IS NOT NULL THEN 'DELETED'
				WHEN b.status = true THEN 'ACTIVE'
				ELSE 'SUSPENDED'
			END,
			COALESCE(b.user_id::text, ''),
			COALESCE(bu.email, ''),
			r.user_id::text,
			COALESCE(ru.email, ''),
			r.reason, r.additional_comments, r.report_status, r.created_at
		FROM business_reports r
		LEFT JOIN business_profiles b ON r.business_id = b.id
		LEFT JOIN users bu ON b.user_id = bu.id
		LEFT JOIN users ru ON r.user_id = ru.id
		WHERE r.id = $1
	`
	report := &models.AdminBusinessReportResponse{}
	err := r.db.Pool.QueryRow(ctx, query, reportID).Scan(
		&report.ID, &report.BusinessID, &report.BusinessName, &report.BusinessStatus,
		&report.BusinessOwnerID, &report.BusinessOwnerEmail,
		&report.ReporterID, &report.ReporterEmail,
		&report.Reason, &report.AdditionalComments, &report.Status, &report.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return report, nil
}

func (r *adminRepository) UpdatePostReportStatus(ctx context.Context, reportID, status string) error {
	query := `UPDATE post_reports SET report_status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, status, reportID)
	return err
}

func (r *adminRepository) UpdateCommentReportStatus(ctx context.Context, reportID, status string) error {
	query := `UPDATE comment_reports SET report_status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, status, reportID)
	return err
}

func (r *adminRepository) UpdateUserReportResolved(ctx context.Context, reportID string, resolved bool) error {
	query := `UPDATE user_reports SET resolved = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, resolved, reportID)
	return err
}

func (r *adminRepository) UpdateBusinessReportStatus(ctx context.Context, reportID, status string) error {
	query := `UPDATE business_reports SET report_status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, status, reportID)
	return err
}

func (r *adminRepository) GetAllFCMTokens(ctx context.Context) ([]string, error) {
	query := `SELECT token FROM fcm_tokens`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (r *adminRepository) GetFCMTokensByProvince(ctx context.Context, province string) ([]string, error) {
	query := `
		SELECT ft.token 
		FROM fcm_tokens ft
		JOIN profiles p ON ft.user_id = p.id
		WHERE p.province = $1
	`
	rows, err := r.db.Pool.Query(ctx, query, province)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (r *adminRepository) GetFCMTokensByUserIDs(ctx context.Context, userIDs []string) ([]string, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	
	query := `SELECT token FROM fcm_tokens WHERE user_id = ANY($1)`
	rows, err := r.db.Pool.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func (r *adminRepository) ListFeedback(ctx context.Context, filter *models.AdminFeedbackFilter) ([]*models.AdminFeedbackResponse, int64, error) {
	limit := 20
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	offset := (page - 1) * limit

	var countArgs []interface{}
	countConditions := "1=1"
	argIdx := 1
	if filter.Type != "" {
		countConditions = fmt.Sprintf("f.type = $%d", argIdx)
		countArgs = append(countArgs, filter.Type)
		argIdx++
	}
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM user_feedback f WHERE %s`, countConditions)
	var totalCount int64
	err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	args := make([]interface{}, 0, 4)
	whereClause := "1=1"
	argIndex := 1
	if filter.Type != "" {
		whereClause = fmt.Sprintf("f.type = $%d", argIndex)
		args = append(args, filter.Type)
		argIndex++
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT f.id, f.user_id, COALESCE(u.email, ''), f.rating, f.type, f.message, f.app_version, f.device_info, f.created_at
		FROM user_feedback f
		LEFT JOIN users u ON f.user_id = u.id
		WHERE %s
		ORDER BY f.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.AdminFeedbackResponse
	for rows.Next() {
		var f models.AdminFeedbackResponse
		err := rows.Scan(&f.ID, &f.UserID, &f.UserEmail, &f.Rating, &f.Type, &f.Message, &f.AppVersion, &f.DeviceInfo, &f.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, &f)
	}
	return items, totalCount, nil
}

func (r *adminRepository) getPeriodInterval(period string) string {
	switch period {
	case "week":
		return "7 days"
	case "month":
		return "30 days"
	case "year":
		return "365 days"
	default:
		return "30 days"
	}
}

package services

import (
	"context"
	"time"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/notification"
	"go.uber.org/zap"
)

// AdminService handles admin business logic
type AdminService struct {
	adminRepo   repositories.AdminRepository
	fcmClient   *notification.FCMClient
	logger      *zap.Logger
}

// NewAdminService creates a new admin service
func NewAdminService(
	adminRepo repositories.AdminRepository,
	fcmClient *notification.FCMClient,
	logger *zap.Logger,
) *AdminService {
	return &AdminService{
		adminRepo:   adminRepo,
		fcmClient:   fcmClient,
		logger:      logger,
	}
}

// GetDashboardStats retrieves dashboard statistics
func (s *AdminService) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	stats, err := s.adminRepo.GetDashboardStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get dashboard stats", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get dashboard stats", err)
	}
	return stats, nil
}

// GetUserAnalytics retrieves user analytics
func (s *AdminService) GetUserAnalytics(ctx context.Context, period string) (*models.UserAnalytics, error) {
	analytics, err := s.adminRepo.GetUserAnalytics(ctx, period)
	if err != nil {
		s.logger.Error("Failed to get user analytics", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get user analytics", err)
	}
	return analytics, nil
}

// GetPostAnalytics retrieves post analytics
func (s *AdminService) GetPostAnalytics(ctx context.Context, period string) (*models.PostAnalytics, error) {
	analytics, err := s.adminRepo.GetPostAnalytics(ctx, period)
	if err != nil {
		s.logger.Error("Failed to get post analytics", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get post analytics", err)
	}
	return analytics, nil
}

// GetEngagementAnalytics retrieves engagement analytics
func (s *AdminService) GetEngagementAnalytics(ctx context.Context, period string) (*models.EngagementAnalytics, error) {
	analytics, err := s.adminRepo.GetEngagementAnalytics(ctx, period)
	if err != nil {
		s.logger.Error("Failed to get engagement analytics", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get engagement analytics", err)
	}
	return analytics, nil
}

// ListUsers lists users with filtering and pagination
func (s *AdminService) ListUsers(ctx context.Context, filter *models.AdminUserFilter) (*models.PaginatedResponse, error) {
	users, total, err := s.adminRepo.ListUsers(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list users", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list users", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      users,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetUser gets a user by ID
func (s *AdminService) GetUser(ctx context.Context, userID string) (*models.AdminUserResponse, error) {
	user, err := s.adminRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewNotFoundError("User not found", err)
	}
	return user, nil
}

// GetUserDetail gets full user details including posts and businesses
func (s *AdminService) GetUserDetail(ctx context.Context, userID string) (*models.AdminUserDetailResponse, error) {
	user, err := s.adminRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewNotFoundError("User not found", err)
	}

	bio, _ := s.adminRepo.GetUserBio(ctx, userID)

	posts, err := s.adminRepo.GetUserPosts(ctx, userID, 10)
	if err != nil {
		s.logger.Error("Failed to get user posts", zap.String("user_id", userID), zap.Error(err))
		posts = []*models.AdminPostResponse{}
	}

	businesses, err := s.adminRepo.GetUserBusinesses(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user businesses", zap.String("user_id", userID), zap.Error(err))
		businesses = []*models.AdminBusinessResponse{}
	}

	// Convert slice of pointers to slice of values
	postsVal := make([]models.AdminPostResponse, len(posts))
	for i, p := range posts {
		postsVal[i] = *p
	}
	businessesVal := make([]models.AdminBusinessResponse, len(businesses))
	for i, b := range businesses {
		businessesVal[i] = *b
	}

	return &models.AdminUserDetailResponse{
		AdminUserResponse: *user,
		Bio:               bio,
		BusinessCount:     int64(len(businesses)),
		RecentPosts:       postsVal,
		Businesses:        businessesVal,
	}, nil
}

// SuspendUser suspends a user for a specified number of days
func (s *AdminService) SuspendUser(ctx context.Context, userID string, days int, reason string, adminID string) error {
	until := time.Now().AddDate(0, 0, days)
	
	err := s.adminRepo.SuspendUser(ctx, userID, until)
	if err != nil {
		s.logger.Error("Failed to suspend user", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to suspend user", err)
	}
	
	s.logger.Info("User suspended",
		zap.String("user_id", userID),
		zap.String("admin_id", adminID),
		zap.Int("days", days),
		zap.String("reason", reason),
		zap.Time("until", until),
	)
	
	return nil
}

// UnsuspendUser removes suspension from a user
func (s *AdminService) UnsuspendUser(ctx context.Context, userID string, adminID string) error {
	err := s.adminRepo.UnsuspendUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to unsuspend user", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to unsuspend user", err)
	}
	
	s.logger.Info("User unsuspended",
		zap.String("user_id", userID),
		zap.String("admin_id", adminID),
	)
	
	return nil
}

// UpdateUserRole updates a user's role
func (s *AdminService) UpdateUserRole(ctx context.Context, userID string, role string, adminID string) error {
	userRole := models.UserRole(role)
	
	err := s.adminRepo.UpdateUserRole(ctx, userID, userRole)
	if err != nil {
		s.logger.Error("Failed to update user role", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to update user role", err)
	}
	
	s.logger.Info("User role updated",
		zap.String("user_id", userID),
		zap.String("admin_id", adminID),
		zap.String("new_role", role),
	)
	
	return nil
}

// DeleteUser soft deletes a user
func (s *AdminService) DeleteUser(ctx context.Context, userID string, adminID string) error {
	err := s.adminRepo.DeleteUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to delete user", zap.String("user_id", userID), zap.Error(err))
		return utils.NewInternalError("Failed to delete user", err)
	}
	
	s.logger.Info("User deleted",
		zap.String("user_id", userID),
		zap.String("admin_id", adminID),
	)
	
	return nil
}

// ListPosts lists posts with filtering and pagination
func (s *AdminService) ListPosts(ctx context.Context, filter *models.AdminPostFilter) (*models.PaginatedResponse, error) {
	posts, total, err := s.adminRepo.ListPosts(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list posts", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list posts", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      posts,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetPostDetail retrieves full post details with comments
func (s *AdminService) GetPostDetail(ctx context.Context, postID string) (*models.AdminPostDetailResponse, error) {
	post, err := s.adminRepo.GetPostByID(ctx, postID)
	if err != nil {
		s.logger.Error("Failed to get post detail", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewNotFoundError("Post not found", err)
	}

	comments, err := s.adminRepo.GetPostComments(ctx, postID)
	if err != nil {
		s.logger.Error("Failed to get post comments", zap.String("post_id", postID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get post comments", err)
	}
	post.Comments = comments

	return post, nil
}

// UpdatePostStatus updates a post's status
func (s *AdminService) UpdatePostStatus(ctx context.Context, postID, status, adminID string) error {
	err := s.adminRepo.UpdatePostStatus(ctx, postID, status)
	if err != nil {
		s.logger.Error("Failed to update post status", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to update post status", err)
	}
	
	s.logger.Info("Post status updated",
		zap.String("post_id", postID),
		zap.String("admin_id", adminID),
		zap.String("status", status),
	)
	
	return nil
}

// DeletePost soft deletes a post
func (s *AdminService) DeletePost(ctx context.Context, postID, adminID string) error {
	err := s.adminRepo.DeletePost(ctx, postID)
	if err != nil {
		s.logger.Error("Failed to delete post", zap.String("post_id", postID), zap.Error(err))
		return utils.NewInternalError("Failed to delete post", err)
	}
	
	s.logger.Info("Post deleted",
		zap.String("post_id", postID),
		zap.String("admin_id", adminID),
	)
	
	return nil
}

// ListComments lists comments with filtering and pagination
func (s *AdminService) ListComments(ctx context.Context, filter *models.AdminCommentFilter) (*models.PaginatedResponse, error) {
	comments, total, err := s.adminRepo.ListComments(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list comments", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list comments", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      comments,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// DeleteComment soft deletes a comment and marks any related comment reports as RESOLVED
func (s *AdminService) DeleteComment(ctx context.Context, commentID, adminID string) error {
	err := s.adminRepo.DeleteComment(ctx, commentID)
	if err != nil {
		s.logger.Error("Failed to delete comment", zap.String("comment_id", commentID), zap.Error(err))
		return utils.NewInternalError("Failed to delete comment", err)
	}
	// Mark all reports for this comment as resolved
	if err := s.adminRepo.ResolveCommentReportsByCommentID(ctx, commentID); err != nil {
		s.logger.Warn("Failed to resolve comment reports for deleted comment", zap.String("comment_id", commentID), zap.Error(err))
		// non-fatal: comment was already deleted
	}
	s.logger.Info("Comment deleted",
		zap.String("comment_id", commentID),
		zap.String("admin_id", adminID),
	)
	return nil
}

// RestoreComment unhides a soft-deleted comment (clears deleted_at)
func (s *AdminService) RestoreComment(ctx context.Context, commentID, adminID string) error {
	err := s.adminRepo.RestoreComment(ctx, commentID)
	if err != nil {
		s.logger.Error("Failed to restore comment", zap.String("comment_id", commentID), zap.Error(err))
		return utils.NewInternalError("Failed to restore comment", err)
	}
	s.logger.Info("Comment restored",
		zap.String("comment_id", commentID),
		zap.String("admin_id", adminID),
	)
	return nil
}

// ListBusinesses lists businesses with filtering and pagination
func (s *AdminService) ListBusinesses(ctx context.Context, filter *models.AdminBusinessFilter) (*models.PaginatedResponse, error) {
	businesses, total, err := s.adminRepo.ListBusinesses(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list businesses", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list businesses", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      businesses,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetBusinessDetail retrieves full business details including posts
func (s *AdminService) GetBusinessDetail(ctx context.Context, businessID string) (*models.AdminBusinessDetailResponse, error) {
	business, err := s.adminRepo.GetBusinessByID(ctx, businessID)
	if err != nil {
		s.logger.Error("Failed to get business", zap.String("business_id", businessID), zap.Error(err))
		return nil, utils.NewNotFoundError("Business not found", err)
	}

	hours, _ := s.adminRepo.GetBusinessHours(ctx, businessID)
	categories, _ := s.adminRepo.GetBusinessCategories(ctx, businessID)
	gallery, _ := s.adminRepo.GetBusinessGallery(ctx, businessID)

	posts, err := s.adminRepo.GetBusinessPosts(ctx, businessID, 10)
	if err != nil {
		s.logger.Error("Failed to get business posts", zap.String("business_id", businessID), zap.Error(err))
		posts = []*models.AdminPostResponse{}
	}

	// Convert slice of pointers to slice of values
	postsVal := make([]models.AdminPostResponse, len(posts))
	for i, p := range posts {
		postsVal[i] = *p
	}

	business.Hours = hours
	business.Categories = categories
	business.Gallery = gallery
	business.RecentPosts = postsVal

	return business, nil
}

// UpdateBusinessStatus updates a business's status
func (s *AdminService) UpdateBusinessStatus(ctx context.Context, businessID, status, adminID string) error {
	err := s.adminRepo.UpdateBusinessStatus(ctx, businessID, status)
	if err != nil {
		s.logger.Error("Failed to update business status", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to update business status", err)
	}
	
	s.logger.Info("Business status updated",
		zap.String("business_id", businessID),
		zap.String("admin_id", adminID),
		zap.String("status", status),
	)
	
	return nil
}

// DeleteBusiness soft deletes a business
func (s *AdminService) DeleteBusiness(ctx context.Context, businessID, adminID string) error {
	err := s.adminRepo.DeleteBusiness(ctx, businessID)
	if err != nil {
		s.logger.Error("Failed to delete business", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to delete business", err)
	}
	
	s.logger.Info("Business deleted",
		zap.String("business_id", businessID),
		zap.String("admin_id", adminID),
	)
	
	return nil
}

// ListPostReports lists post reports with filtering and pagination
func (s *AdminService) ListPostReports(ctx context.Context, filter *models.AdminReportFilter) (*models.PaginatedResponse, error) {
	reports, total, err := s.adminRepo.ListPostReports(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list post reports", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list post reports", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      reports,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetPostReport returns a single post report by ID
func (s *AdminService) GetPostReport(ctx context.Context, reportID string) (*models.AdminPostReportResponse, error) {
	report, err := s.adminRepo.GetPostReportByID(ctx, reportID)
	if err != nil {
		s.logger.Error("Failed to get post report", zap.String("report_id", reportID), zap.Error(err))
		return nil, utils.NewNotFoundError("Post report not found", err)
	}
	return report, nil
}

// ListCommentReports lists comment reports with filtering and pagination
func (s *AdminService) ListCommentReports(ctx context.Context, filter *models.AdminReportFilter) (*models.PaginatedResponse, error) {
	reports, total, err := s.adminRepo.ListCommentReports(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list comment reports", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list comment reports", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      reports,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetCommentReport returns a single comment report by ID
func (s *AdminService) GetCommentReport(ctx context.Context, reportID string) (*models.AdminCommentReportResponse, error) {
	report, err := s.adminRepo.GetCommentReportByID(ctx, reportID)
	if err != nil {
		s.logger.Error("Failed to get comment report", zap.String("report_id", reportID), zap.Error(err))
		return nil, utils.NewNotFoundError("Comment report not found", err)
	}
	return report, nil
}

// ListUserReports lists user reports with filtering and pagination
func (s *AdminService) ListUserReports(ctx context.Context, filter *models.AdminReportFilter) (*models.PaginatedResponse, error) {
	reports, total, err := s.adminRepo.ListUserReports(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list user reports", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list user reports", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      reports,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetUserReport returns a single user report by ID
func (s *AdminService) GetUserReport(ctx context.Context, reportID string) (*models.AdminUserReportResponse, error) {
	report, err := s.adminRepo.GetUserReportByID(ctx, reportID)
	if err != nil {
		s.logger.Error("Failed to get user report", zap.String("report_id", reportID), zap.Error(err))
		return nil, utils.NewNotFoundError("User report not found", err)
	}
	return report, nil
}

// ListBusinessReports lists business reports with filtering and pagination
func (s *AdminService) ListBusinessReports(ctx context.Context, filter *models.AdminReportFilter) (*models.PaginatedResponse, error) {
	reports, total, err := s.adminRepo.ListBusinessReports(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list business reports", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list business reports", err)
	}
	
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	page := 1
	if filter.Page > 0 {
		page = filter.Page
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	
	return &models.PaginatedResponse{
		Items:      reports,
		TotalCount: total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetBusinessReport returns a single business report by ID
func (s *AdminService) GetBusinessReport(ctx context.Context, reportID string) (*models.AdminBusinessReportResponse, error) {
	report, err := s.adminRepo.GetBusinessReportByID(ctx, reportID)
	if err != nil {
		s.logger.Error("Failed to get business report", zap.String("report_id", reportID), zap.Error(err))
		return nil, utils.NewNotFoundError("Business report not found", err)
	}
	return report, nil
}

// UpdateReportStatus updates a report's status based on type
func (s *AdminService) UpdateReportStatus(ctx context.Context, reportType, reportID, status, adminID string) error {
	var err error
	
	switch reportType {
	case "posts":
		err = s.adminRepo.UpdatePostReportStatus(ctx, reportID, status)
	case "comments":
		err = s.adminRepo.UpdateCommentReportStatus(ctx, reportID, status)
	case "users":
		resolved := status == "RESOLVED"
		err = s.adminRepo.UpdateUserReportResolved(ctx, reportID, resolved)
	case "businesses":
		err = s.adminRepo.UpdateBusinessReportStatus(ctx, reportID, status)
	default:
		return utils.NewBadRequestError("Invalid report type", nil)
	}
	
	if err != nil {
		s.logger.Error("Failed to update report status",
			zap.String("report_type", reportType),
			zap.String("report_id", reportID),
			zap.Error(err),
		)
		return utils.NewInternalError("Failed to update report status", err)
	}
	
	s.logger.Info("Report status updated",
		zap.String("report_type", reportType),
		zap.String("report_id", reportID),
		zap.String("admin_id", adminID),
		zap.String("status", status),
	)
	
	return nil
}

// BroadcastNotification sends a notification to multiple users
func (s *AdminService) BroadcastNotification(ctx context.Context, req *models.BroadcastNotificationRequest, adminID string) error {
	if s.fcmClient == nil {
		return utils.NewInternalError("Push notifications are not configured", nil)
	}
	
	var tokens []string
	var err error
	
	if len(req.UserIDs) > 0 {
		tokens, err = s.adminRepo.GetFCMTokensByUserIDs(ctx, req.UserIDs)
	} else if req.Province != nil && *req.Province != "" {
		tokens, err = s.adminRepo.GetFCMTokensByProvince(ctx, *req.Province)
	} else {
		tokens, err = s.adminRepo.GetAllFCMTokens(ctx)
	}
	
	if err != nil {
		s.logger.Error("Failed to get FCM tokens", zap.Error(err))
		return utils.NewInternalError("Failed to get notification targets", err)
	}
	
	if len(tokens) == 0 {
		s.logger.Warn("No FCM tokens found for broadcast")
		return nil
	}
	
	successCount := 0
	failCount := 0
	
	payload := &notification.PushPayload{
		Title: req.Title,
		Body:  req.Message,
	}
	
	for _, token := range tokens {
		err := s.fcmClient.SendNotification(ctx, token, payload)
		if err != nil {
			failCount++
			s.logger.Warn("Failed to send notification",
				zap.String("token", token[:10]+"..."),
				zap.Error(err),
			)
		} else {
			successCount++
		}
	}
	
	s.logger.Info("Broadcast notification sent",
		zap.String("admin_id", adminID),
		zap.String("title", req.Title),
		zap.Int("success_count", successCount),
		zap.Int("fail_count", failCount),
		zap.Int("total_tokens", len(tokens)),
	)
	
	return nil
}

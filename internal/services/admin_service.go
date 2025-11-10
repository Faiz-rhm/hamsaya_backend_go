package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"go.uber.org/zap"
)

// AdminService handles admin-related business logic
type AdminService struct {
	adminRepo repositories.AdminRepository
	logger    *zap.SugaredLogger
}

// NewAdminService creates a new admin service
func NewAdminService(adminRepo repositories.AdminRepository, logger *zap.Logger) *AdminService {
	return &AdminService{
		adminRepo: adminRepo,
		logger:    logger.Sugar(),
	}
}

// GetStatistics retrieves dashboard statistics for admin
func (s *AdminService) GetStatistics(ctx context.Context) (*models.AdminStatistics, error) {
	s.logger.Info("Fetching admin statistics")

	stats, err := s.adminRepo.GetStatistics(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get admin statistics", "error", err)
		return nil, err
	}

	s.logger.Infow("Admin statistics fetched successfully",
		"total_active_accounts", stats.TotalActiveAccounts,
		"deactivated_accounts", stats.DeactivatedAccounts,
		"recently_active_users", stats.RecentlyActiveUsers,
		"dormant_users", stats.DormantUsers,
		"total_posts", stats.TotalPosts,
		"total_businesses", stats.TotalBusinesses,
		"pending_reports", stats.PendingReports.Total,
	)

	return stats, nil
}

// ListUsers retrieves a paginated list of users with optional filtering
func (s *AdminService) ListUsers(ctx context.Context, search string, isActive *bool, page, limit int) ([]models.AdminUserListItem, int64, error) {
	s.logger.Infow("Listing users",
		"search", search,
		"is_active", isActive,
		"page", page,
		"limit", limit,
	)

	users, totalCount, err := s.adminRepo.ListUsers(ctx, search, isActive, page, limit)
	if err != nil {
		s.logger.Errorw("Failed to list users", "error", err)
		return nil, 0, err
	}

	s.logger.Infow("Users listed successfully",
		"count", len(users),
		"total", totalCount,
	)

	return users, totalCount, nil
}

// UpdateUserStatus updates a user's active status
func (s *AdminService) UpdateUserStatus(ctx context.Context, userID string, isActive bool) error {
	s.logger.Infow("Updating user status",
		"user_id", userID,
		"is_active", isActive,
	)

	err := s.adminRepo.UpdateUserStatus(ctx, userID, isActive)
	if err != nil {
		s.logger.Errorw("Failed to update user status",
			"user_id", userID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("User status updated successfully",
		"user_id", userID,
		"is_active", isActive,
	)

	return nil
}

// ListPosts retrieves a paginated list of posts with optional filtering
func (s *AdminService) ListPosts(ctx context.Context, postType, search string, page, limit int) ([]models.AdminPostListItem, int64, error) {
	s.logger.Infow("Listing posts",
		"type", postType,
		"search", search,
		"page", page,
		"limit", limit,
	)

	posts, totalCount, err := s.adminRepo.ListPosts(ctx, postType, search, page, limit)
	if err != nil {
		s.logger.Errorw("Failed to list posts", "error", err)
		return nil, 0, err
	}

	s.logger.Infow("Posts listed successfully",
		"count", len(posts),
		"total", totalCount,
	)

	return posts, totalCount, nil
}

// ListReports retrieves a paginated list of reports with optional filtering
func (s *AdminService) ListReports(ctx context.Context, reportType, status, search string, page, limit int) ([]models.AdminReportListItem, int64, error) {
	s.logger.Infow("Listing reports",
		"type", reportType,
		"status", status,
		"search", search,
		"page", page,
		"limit", limit,
	)

	reports, totalCount, err := s.adminRepo.ListReports(ctx, reportType, status, search, page, limit)
	if err != nil {
		s.logger.Errorw("Failed to list reports", "error", err)
		return nil, 0, err
	}

	s.logger.Infow("Reports listed successfully",
		"count", len(reports),
		"total", totalCount,
	)

	return reports, totalCount, nil
}

// UpdateReportStatus updates the status of a report
func (s *AdminService) UpdateReportStatus(ctx context.Context, reportType, reportID, status string) error {
	s.logger.Infow("Updating report status",
		"report_id", reportID,
		"report_type", reportType,
		"status", status,
	)

	err := s.adminRepo.UpdateReportStatus(ctx, reportType, reportID, status)
	if err != nil {
		s.logger.Errorw("Failed to update report status",
			"report_id", reportID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("Report status updated successfully",
		"report_id", reportID,
		"status", status,
	)

	return nil
}

// ListBusinesses retrieves a paginated list of businesses with optional filtering
func (s *AdminService) ListBusinesses(ctx context.Context, search string, status *bool, page, limit int) ([]models.AdminBusinessListItem, int64, error) {
	s.logger.Infow("Listing businesses",
		"search", search,
		"status", status,
		"page", page,
		"limit", limit,
	)

	businesses, totalCount, err := s.adminRepo.ListBusinesses(ctx, search, status, page, limit)
	if err != nil {
		s.logger.Errorw("Failed to list businesses", "error", err)
		return nil, 0, err
	}

	s.logger.Infow("Businesses listed successfully",
		"count", len(businesses),
		"total", totalCount,
	)

	return businesses, totalCount, nil
}

// UpdateBusinessStatus updates a business's active status
func (s *AdminService) UpdateBusinessStatus(ctx context.Context, businessID string, status bool) error {
	s.logger.Infow("Updating business status",
		"business_id", businessID,
		"status", status,
	)

	err := s.adminRepo.UpdateBusinessStatus(ctx, businessID, status)
	if err != nil {
		s.logger.Errorw("Failed to update business status",
			"business_id", businessID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("Business status updated successfully",
		"business_id", businessID,
		"status", status,
	)

	return nil
}

// UpdatePostStatus updates a post's active status
func (s *AdminService) UpdatePostStatus(ctx context.Context, postID string, status bool) error {
	s.logger.Infow("Updating post status",
		"post_id", postID,
		"status", status,
	)

	err := s.adminRepo.UpdatePostStatus(ctx, postID, status)
	if err != nil {
		s.logger.Errorw("Failed to update post status",
			"post_id", postID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("Post status updated successfully",
		"post_id", postID,
		"status", status,
	)

	return nil
}

// UpdateUser updates user information (admin operation)
func (s *AdminService) UpdateUser(ctx context.Context, userID string, req *models.AdminUpdateUserRequest) error {
	s.logger.Infow("Updating user information",
		"user_id", userID,
		"fields", req,
	)

	err := s.adminRepo.UpdateUser(ctx, userID, req)
	if err != nil {
		s.logger.Errorw("Failed to update user",
			"user_id", userID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("User updated successfully",
		"user_id", userID,
	)

	return nil
}

// UpdatePost updates post information (admin operation)
func (s *AdminService) UpdatePost(ctx context.Context, postID string, req *models.AdminUpdatePostRequest) error {
	s.logger.Infow("Updating post information",
		"post_id", postID,
		"fields", req,
	)

	err := s.adminRepo.UpdatePost(ctx, postID, req)
	if err != nil {
		s.logger.Errorw("Failed to update post",
			"post_id", postID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("Post updated successfully",
		"post_id", postID,
	)

	return nil
}

// UpdateBusiness updates business information (admin operation)
func (s *AdminService) UpdateBusiness(ctx context.Context, businessID string, req *models.AdminUpdateBusinessRequest) error {
	s.logger.Infow("Updating business information",
		"business_id", businessID,
		"fields", req,
	)

	err := s.adminRepo.UpdateBusiness(ctx, businessID, req)
	if err != nil {
		s.logger.Errorw("Failed to update business",
			"business_id", businessID,
			"error", err,
		)
		return err
	}

	s.logger.Infow("Business updated successfully",
		"business_id", businessID,
	)

	return nil
}

// GetSellPostStatistics retrieves statistics for SELL type posts
func (s *AdminService) GetSellPostStatistics(ctx context.Context) (*models.SellStatistics, error) {
	s.logger.Infow("Getting sell post statistics")

	stats, err := s.adminRepo.GetSellPostStatistics(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get sell post statistics", "error", err)
		return nil, err
	}

	s.logger.Infow("Sell post statistics retrieved successfully")
	return stats, nil
}

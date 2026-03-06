package services

import (
	"context"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// ReportService handles report-related business logic
type ReportService struct {
	reportRepo repositories.ReportRepository
	postRepo   repositories.PostRepository
	userRepo   repositories.UserRepository
	validator  *utils.Validator
	logger     *zap.SugaredLogger
}

// NewReportService creates a new report service
func NewReportService(
	reportRepo repositories.ReportRepository,
	postRepo repositories.PostRepository,
	userRepo repositories.UserRepository,
	validator *utils.Validator,
) *ReportService {
	return &ReportService{
		reportRepo: reportRepo,
		postRepo:   postRepo,
		userRepo:   userRepo,
		validator:  validator,
		logger:     utils.GetLogger(),
	}
}

// ReportPost creates a report for a post
func (s *ReportService) ReportPost(ctx context.Context, userID, postID string, req *models.CreatePostReportRequest) error {
	s.logger.Infow("Processing post report request",
		"user_id", userID,
		"post_id", postID,
		"reason", req.Reason,
	)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		s.logger.Warnw("Post report validation failed", "user_id", userID, "error", err)
		return utils.NewBadRequestError("Invalid request", err)
	}

	// Check if post exists
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		s.logger.Errorw("Failed to find post for reporting", "post_id", postID, "error", err)
		return utils.NewNotFoundError("Post not found", err)
	}
	if post == nil {
		s.logger.Warnw("Post not found for reporting", "post_id", postID)
		return utils.NewNotFoundError("Post not found", nil)
	}

	// Don't allow reporting own posts
	if post.UserID != nil && *post.UserID == userID {
		s.logger.Warnw("User attempted to report own post", "user_id", userID, "post_id", postID)
		return utils.NewBadRequestError("Cannot report your own post", nil)
	}

	// Create report
	report := &models.PostReport{
		UserID:             userID,
		PostID:             postID,
		Reason:             req.Reason,
		AdditionalComments: req.AdditionalComments,
		ReportStatus:       models.ReportStatusPending,
	}

	if err := s.reportRepo.CreatePostReport(ctx, report); err != nil {
		s.logger.Errorw("Failed to create post report", "user_id", userID, "post_id", postID, "error", err)
		return utils.NewInternalServerError("Failed to create report", err)
	}

	s.logger.Infow("Post report created successfully", "user_id", userID, "post_id", postID)
	return nil
}

// ReportComment creates a report for a comment
func (s *ReportService) ReportComment(ctx context.Context, userID, commentID string, req *models.CreateCommentReportRequest) error {
	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return utils.NewBadRequestError("Invalid request", err)
	}

	// Create report
	report := &models.CommentReport{
		UserID:             userID,
		CommentID:          commentID,
		Reason:             req.Reason,
		AdditionalComments: req.AdditionalComments,
		ReportStatus:       models.ReportStatusPending,
	}

	if err := s.reportRepo.CreateCommentReport(ctx, report); err != nil {
		return utils.NewInternalServerError("Failed to create report", err)
	}

	return nil
}

// ReportUser creates a report for a user
func (s *ReportService) ReportUser(ctx context.Context, reporterID, reportedUserID string, req *models.CreateUserReportRequest) error {
	s.logger.Infow("Processing user report request",
		"reporter_id", reporterID,
		"reported_user_id", reportedUserID,
		"reason", req.Reason,
	)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		s.logger.Warnw("User report validation failed", "reporter_id", reporterID, "error", err)
		return utils.NewBadRequestError("Invalid request", err)
	}

	// Don't allow reporting yourself
	if reporterID == reportedUserID {
		s.logger.Warnw("User attempted to report themselves", "user_id", reporterID)
		return utils.NewBadRequestError("Cannot report yourself", nil)
	}

	// Check if reported user exists
	user, err := s.userRepo.GetByID(ctx, reportedUserID)
	if err != nil {
		s.logger.Errorw("Failed to find reported user", "user_id", reportedUserID, "error", err)
		return utils.NewNotFoundError("User not found", err)
	}
	if user == nil {
		s.logger.Warnw("Reported user not found", "user_id", reportedUserID)
		return utils.NewNotFoundError("User not found", nil)
	}

	// Create report
	report := &models.UserReport{
		ReportedUser: reportedUserID,
		ReportedByID: reporterID,
		Reason:       req.Reason,
		Description:  req.Description,
		Resolved:     false,
	}

	if err := s.reportRepo.CreateUserReport(ctx, report); err != nil {
		s.logger.Errorw("Failed to create user report", "reporter_id", reporterID, "reported_user_id", reportedUserID, "error", err)
		return utils.NewInternalServerError("Failed to create report", err)
	}

	s.logger.Infow("User report created successfully", "reporter_id", reporterID, "reported_user_id", reportedUserID)
	return nil
}

// ReportBusiness creates a report for a business
func (s *ReportService) ReportBusiness(ctx context.Context, userID, businessID string, req *models.CreateBusinessReportRequest) error {
	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return utils.NewBadRequestError("Invalid request", err)
	}

	// Create report
	report := &models.BusinessReport{
		BusinessID:         businessID,
		UserID:             userID,
		Reason:             req.Reason,
		AdditionalComments: req.AdditionalComments,
		ReportStatus:       models.ReportStatusPending,
	}

	if err := s.reportRepo.CreateBusinessReport(ctx, report); err != nil {
		return utils.NewInternalServerError("Failed to create report", err)
	}

	return nil
}

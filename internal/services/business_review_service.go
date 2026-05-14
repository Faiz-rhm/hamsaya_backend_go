package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// BusinessReviewService coordinates review writes (validation, ownership checks)
// and reads (list + stats). Aggregates are maintained by a DB trigger so this
// service never recomputes them.
type BusinessReviewService struct {
	reviewRepo          repositories.BusinessReviewRepository
	businessRepo        repositories.BusinessRepository
	userRepo            repositories.UserRepository
	notificationService *NotificationService
	logger              *zap.Logger
}

// NewBusinessReviewService wires the review service.
func NewBusinessReviewService(
	reviewRepo repositories.BusinessReviewRepository,
	businessRepo repositories.BusinessRepository,
	userRepo repositories.UserRepository,
	notificationService *NotificationService,
	logger *zap.Logger,
) *BusinessReviewService {
	return &BusinessReviewService{
		reviewRepo:          reviewRepo,
		businessRepo:        businessRepo,
		userRepo:            userRepo,
		notificationService: notificationService,
		logger:              logger,
	}
}

// Submit creates or replaces a user's review for a business.
// Self-reviews (business owner reviewing their own business) are rejected.
func (s *BusinessReviewService) Submit(ctx context.Context, businessID, userID string, req *models.CreateBusinessReviewRequest) (*models.BusinessReview, error) {
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, utils.NewNotFoundError("Business profile not found", err)
	}
	if business.UserID == userID {
		return nil, utils.NewBadRequestError("You cannot review your own business", nil)
	}

	review := &models.BusinessReview{
		ID:                uuid.NewString(),
		BusinessProfileID: businessID,
		UserID:            userID,
		Rating:            req.Rating,
		Comment:           req.Comment,
	}
	if err := s.reviewRepo.Upsert(ctx, review); err != nil {
		s.logger.Error("Failed to upsert review", zap.Error(err))
		return nil, utils.NewInternalError("Failed to submit review", err)
	}

	// Best-effort notification to the business owner. Failures are logged
	// only — they must not block the review write.
	go s.notifyOwner(business, userID, review)

	return review, nil
}

// Update edits an existing review. Only the author may edit.
func (s *BusinessReviewService) Update(ctx context.Context, reviewID, userID string, req *models.UpdateBusinessReviewRequest) (*models.BusinessReview, error) {
	updated, err := s.reviewRepo.Update(ctx, reviewID, userID, req.Rating, req.Comment)
	if errors.Is(err, repositories.ErrReviewNotFound) {
		return nil, utils.NewNotFoundError("Review not found", err)
	}
	if err != nil {
		return nil, utils.NewInternalError("Failed to update review", err)
	}
	return updated, nil
}

// Delete removes a review. Owner can delete their own; admins can delete any.
func (s *BusinessReviewService) Delete(ctx context.Context, reviewID, userID string, isAdmin bool) error {
	err := s.reviewRepo.Delete(ctx, reviewID, userID, isAdmin)
	if errors.Is(err, repositories.ErrReviewNotFound) {
		return utils.NewNotFoundError("Review not found", err)
	}
	if err != nil {
		return utils.NewInternalError("Failed to delete review", err)
	}
	return nil
}

// SetHidden toggles moderation visibility (admin-only at the route layer).
func (s *BusinessReviewService) SetHidden(ctx context.Context, reviewID string, hidden bool) error {
	if err := s.reviewRepo.SetHidden(ctx, reviewID, hidden); err != nil {
		if errors.Is(err, repositories.ErrReviewNotFound) {
			return utils.NewNotFoundError("Review not found", err)
		}
		return utils.NewInternalError("Failed to update review visibility", err)
	}
	return nil
}

// GetMyReview returns the calling user's review for a business, or nil.
func (s *BusinessReviewService) GetMyReview(ctx context.Context, businessID, userID string) (*models.BusinessReview, error) {
	review, err := s.reviewRepo.GetByBusinessAndUser(ctx, businessID, userID)
	if err != nil {
		return nil, utils.NewInternalError("Failed to load review", err)
	}
	return review, nil
}

// List returns paginated reviews + total count for a business.
func (s *BusinessReviewService) List(ctx context.Context, businessID string, includeHidden bool, limit, offset int) ([]*models.BusinessReviewWithAuthor, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	reviews, total, err := s.reviewRepo.ListByBusiness(ctx, businessID, includeHidden, limit, offset)
	if err != nil {
		return nil, 0, utils.NewInternalError("Failed to load reviews", err)
	}
	return reviews, total, nil
}

// Stats returns aggregate stats for the summary card.
func (s *BusinessReviewService) Stats(ctx context.Context, businessID string) (*models.BusinessReviewStats, error) {
	stats, err := s.reviewRepo.GetStats(ctx, businessID)
	if err != nil {
		return nil, utils.NewInternalError("Failed to load review stats", err)
	}
	return stats, nil
}

func (s *BusinessReviewService) notifyOwner(business *models.BusinessProfile, reviewerID string, review *models.BusinessReview) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Warn("notifyOwner panicked", zap.Any("recover", r))
		}
	}()

	ctx := context.Background()
	reviewerName := ""
	if actor, err := s.userRepo.GetProfileByUserID(ctx, reviewerID); err == nil && actor != nil {
		if name := actor.FullName(); name != "" {
			reviewerName = name
		}
	}
	title := strings.TrimSpace(reviewerName + " left a review on your business")
	msg := title
	data := map[string]interface{}{
		"actor_id":    reviewerID,
		"actor_name":  reviewerName,
		"business_id": business.ID,
		"review_id":   review.ID,
		"rating":      review.Rating,
	}
	if _, err := s.notificationService.CreateNotification(ctx, &models.CreateNotificationRequest{
		UserID:  business.UserID,
		Type:    models.NotificationTypeBusinessReview,
		Title:   &title,
		Message: &msg,
		Data:    data,
	}); err != nil {
		s.logger.Warn("Failed to notify owner of new review", zap.Error(err))
	}
}

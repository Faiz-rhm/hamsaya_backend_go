package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/cache"
	"go.uber.org/zap"
)

// BusinessVerificationService handles the owner-submit / admin-review flow
// that grants businesses the verified tick.
type BusinessVerificationService struct {
	verificationRepo repositories.BusinessVerificationRepository
	businessRepo     repositories.BusinessRepository
	notification     *NotificationService
	logger           *zap.Logger

	// Optional — the business-profile cache namespace (same one
	// BusinessService uses). Busted on approval so the verified tick shows
	// immediately instead of after the 5-minute profile TTL.
	businessCache *cache.Cache
}

// WithBusinessCache attaches the business-profile cache namespace so
// approvals invalidate cached profiles. Call once at startup.
func (s *BusinessVerificationService) WithBusinessCache(c *cache.Cache) *BusinessVerificationService {
	s.businessCache = c
	return s
}

// NewBusinessVerificationService constructs the service.
func NewBusinessVerificationService(
	verificationRepo repositories.BusinessVerificationRepository,
	businessRepo repositories.BusinessRepository,
	notification *NotificationService,
	logger *zap.Logger,
) *BusinessVerificationService {
	return &BusinessVerificationService{
		verificationRepo: verificationRepo,
		businessRepo:     businessRepo,
		notification:     notification,
		logger:           logger,
	}
}

// Submit files a verification request for a business the caller owns.
// Requires at least one document; rejects when the business is already
// verified or a request is already pending.
func (s *BusinessVerificationService) Submit(
	ctx context.Context, businessID, userID string,
	licenseNo, note *string, documents []models.Photo,
) (*models.BusinessVerificationRequest, error) {
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, utils.NewNotFoundError("Business not found", err)
	}
	if business.UserID != userID {
		return nil, utils.NewUnauthorizedError("You don't own this business", nil)
	}
	if business.IsVerified {
		return nil, utils.NewBadRequestError("Business is already verified", nil)
	}
	if len(documents) == 0 {
		return nil, utils.NewBadRequestError("At least one document is required", nil)
	}

	req := &models.BusinessVerificationRequest{
		ID:         uuid.NewString(),
		BusinessID: businessID,
		UserID:     userID,
		LicenseNo:  licenseNo,
		Note:       note,
		Documents:  documents,
		Status:     models.VerificationStatusPending,
	}
	if err := s.verificationRepo.Create(ctx, req); err != nil {
		if errors.Is(err, repositories.ErrVerificationPending) {
			return nil, utils.NewBadRequestError("A verification request is already pending for this business", err)
		}
		s.logger.Error("Failed to create verification request",
			zap.String("business_id", businessID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to submit verification request", err)
	}

	s.logger.Info("Business verification submitted",
		zap.String("business_id", businessID), zap.String("request_id", req.ID))
	return s.verificationRepo.GetLatestByBusiness(ctx, businessID)
}

// Status returns the latest request for a business (owner only). Nil when the
// owner has never submitted.
func (s *BusinessVerificationService) Status(ctx context.Context, businessID, userID string) (*models.BusinessVerificationRequest, error) {
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, utils.NewNotFoundError("Business not found", err)
	}
	if business.UserID != userID {
		return nil, utils.NewUnauthorizedError("You don't own this business", nil)
	}
	req, err := s.verificationRepo.GetLatestByBusiness(ctx, businessID)
	if errors.Is(err, repositories.ErrVerificationNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, utils.NewInternalError("Failed to load verification status", err)
	}
	return req, nil
}

// List returns the admin queue (status filter optional).
func (s *BusinessVerificationService) List(ctx context.Context, status *string, limit, offset int) ([]*models.BusinessVerificationListItem, int, error) {
	items, total, err := s.verificationRepo.List(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, utils.NewInternalError("Failed to list verification requests", err)
	}
	return items, total, nil
}

// Review approves or rejects a pending request. Approval flips the business's
// verified tick; both outcomes notify the owner.
func (s *BusinessVerificationService) Review(ctx context.Context, requestID, reviewerID, action string, reason *string) (*models.BusinessVerificationRequest, error) {
	req, err := s.verificationRepo.GetByID(ctx, requestID)
	if errors.Is(err, repositories.ErrVerificationNotFound) {
		return nil, utils.NewNotFoundError("Verification request not found", err)
	}
	if err != nil {
		return nil, utils.NewInternalError("Failed to load verification request", err)
	}
	if req.Status != models.VerificationStatusPending {
		return nil, utils.NewBadRequestError("Request has already been reviewed", nil)
	}

	status := models.VerificationStatusRejected
	if action == "approve" {
		status = models.VerificationStatusApproved
	}
	if err := s.verificationRepo.Review(ctx, requestID, reviewerID, status, reason); err != nil {
		s.logger.Error("Failed to review verification request",
			zap.String("request_id", requestID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to review verification request", err)
	}

	if status == models.VerificationStatusApproved {
		if err := s.verificationRepo.SetBusinessVerified(ctx, req.BusinessID, true); err != nil {
			s.logger.Error("Failed to set business verified",
				zap.String("business_id", req.BusinessID), zap.Error(err))
			return nil, utils.NewInternalError("Failed to mark business verified", err)
		}
		// Bust every cached profile variant so the tick is visible on the
		// very next fetch (profile cache TTL is 5 minutes otherwise).
		if s.businessCache != nil {
			s.businessCache.DelPattern(ctx, req.BusinessID+":*")
		}
	}

	// Notify the owner (best-effort).
	s.notifyOwner(ctx, req, status, reason)

	return s.verificationRepo.GetByID(ctx, requestID)
}

func (s *BusinessVerificationService) notifyOwner(ctx context.Context, req *models.BusinessVerificationRequest, status string, reason *string) {
	if s.notification == nil {
		return
	}
	businessName := "Your business"
	if business, err := s.businessRepo.GetByID(ctx, req.BusinessID); err == nil && business.Name != "" {
		businessName = business.Name
	}

	var notifType models.NotificationType
	var title, msg string
	if status == models.VerificationStatusApproved {
		notifType = models.NotificationTypeBusinessVerified
		title = fmt.Sprintf("✔ %s is now verified", businessName)
		msg = "Your business passed verification and now shows a verified badge."
	} else {
		notifType = models.NotificationTypeBusinessVerificationRejected
		title = fmt.Sprintf("%s verification declined", businessName)
		msg = "Your verification request was not approved. You can update your documents and try again."
		if reason != nil && *reason != "" {
			msg = "Reason: " + *reason
		}
	}

	if _, err := s.notification.CreateNotification(ctx, &models.CreateNotificationRequest{
		UserID:  req.UserID,
		Type:    notifType,
		Title:   &title,
		Message: &msg,
		Data: map[string]interface{}{
			"type":        string(notifType),
			"business_id": req.BusinessID,
			"request_id":  req.ID,
		},
	}); err != nil {
		s.logger.Warn("Failed to notify owner of verification outcome", zap.Error(err))
	}
}

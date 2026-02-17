package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// BusinessService handles business profile operations
type BusinessService struct {
	businessRepo repositories.BusinessRepository
	userRepo     repositories.UserRepository
	logger       *zap.Logger
}

// NewBusinessService creates a new business service
func NewBusinessService(
	businessRepo repositories.BusinessRepository,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *BusinessService {
	return &BusinessService{
		businessRepo: businessRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// CreateBusiness creates a new business profile
func (s *BusinessService) CreateBusiness(ctx context.Context, userID string, req *models.CreateBusinessRequest) (*models.BusinessResponse, error) {
	// Create business profile
	businessID := uuid.New().String()
	now := time.Now()

	business := &models.BusinessProfile{
		ID:             businessID,
		UserID:         userID,
		Name:           req.Name,
		LicenseNo:      req.LicenseNo,
		Description:    req.Description,
		Address:        req.Address,
		PhoneNumber:    req.PhoneNumber,
		Email:          req.Email,
		Website:        req.Website,
		Status:         true,
		AdditionalInfo: req.AdditionalInfo,
		Country:        req.Country,
		Province:       req.Province,
		District:       req.District,
		Neighborhood:   req.Neighborhood,
		ShowLocation:   true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Set show location if provided
	if req.ShowLocation != nil {
		business.ShowLocation = *req.ShowLocation
	}

	// Handle location
	if req.Latitude != nil && req.Longitude != nil {
		business.AddressLocation = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	// Create business in database
	if err := s.businessRepo.Create(ctx, business); err != nil {
		s.logger.Error("Failed to create business", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to create business", err)
	}

	// Add categories if provided
	if len(req.CategoryIDs) > 0 {
		if err := s.businessRepo.AddCategories(ctx, businessID, req.CategoryIDs); err != nil {
			s.logger.Error("Failed to add business categories", zap.String("business_id", businessID), zap.Error(err))
			// Continue - don't fail the whole operation
		}
	}

	s.logger.Info("Business created",
		zap.String("business_id", businessID),
		zap.String("user_id", userID),
	)

	// Return enriched business
	return s.GetBusiness(ctx, businessID, &userID)
}

// GetBusiness gets a business profile by ID
func (s *BusinessService) GetBusiness(ctx context.Context, businessID string, viewerID *string) (*models.BusinessResponse, error) {
	// Get business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		s.logger.Warn("Business not found", zap.String("business_id", businessID), zap.Error(err))
		return nil, utils.NewNotFoundError("Business not found", err)
	}

	// Enrich business
	return s.enrichBusiness(ctx, business, viewerID)
}

// GetUserBusinesses gets all businesses for a user
func (s *BusinessService) GetUserBusinesses(ctx context.Context, userID string, limit, offset int) ([]*models.BusinessResponse, error) {
	// Get businesses
	businesses, err := s.businessRepo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get user businesses", zap.String("user_id", userID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get businesses", err)
	}

	// Enrich businesses
	var enrichedBusinesses []*models.BusinessResponse
	for _, business := range businesses {
		enriched, err := s.enrichBusiness(ctx, business, &userID)
		if err != nil {
			s.logger.Warn("Failed to enrich business", zap.String("business_id", business.ID), zap.Error(err))
			continue
		}
		enrichedBusinesses = append(enrichedBusinesses, enriched)
	}

	return enrichedBusinesses, nil
}

// UpdateBusiness updates a business profile
func (s *BusinessService) UpdateBusiness(ctx context.Context, businessID, userID string, req *models.UpdateBusinessRequest) (*models.BusinessResponse, error) {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return nil, utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Update fields
	if req.Name != nil {
		business.Name = *req.Name
	}
	if req.LicenseNo != nil {
		business.LicenseNo = req.LicenseNo
	}
	if req.Description != nil {
		business.Description = req.Description
	}
	if req.Address != nil {
		business.Address = req.Address
	}
	if req.PhoneNumber != nil {
		business.PhoneNumber = req.PhoneNumber
	}
	if req.Email != nil {
		business.Email = req.Email
	}
	if req.Website != nil {
		business.Website = req.Website
	}
	if req.AdditionalInfo != nil {
		business.AdditionalInfo = req.AdditionalInfo
	}
	if req.Status != nil {
		business.Status = *req.Status
	}
	if req.Country != nil {
		business.Country = req.Country
	}
	if req.Province != nil {
		business.Province = req.Province
	}
	if req.District != nil {
		business.District = req.District
	}
	if req.Neighborhood != nil {
		business.Neighborhood = req.Neighborhood
	}
	if req.ShowLocation != nil {
		business.ShowLocation = *req.ShowLocation
	}

	// Handle location update
	if req.Latitude != nil && req.Longitude != nil {
		business.AddressLocation = &pgtype.Point{
			P:     pgtype.Vec2{X: *req.Longitude, Y: *req.Latitude},
			Valid: true,
		}
	}

	business.UpdatedAt = time.Now()

	// Update in database
	if err := s.businessRepo.Update(ctx, business); err != nil {
		s.logger.Error("Failed to update business", zap.String("business_id", businessID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to update business", err)
	}

	// Update categories if provided (category_ids and/or category_names)
	if req.CategoryIDs != nil || (req.CategoryNames != nil && len(req.CategoryNames) > 0) {
		// Remove existing categories
		if err := s.businessRepo.RemoveCategories(ctx, businessID); err != nil {
			s.logger.Error("Failed to remove business categories", zap.String("business_id", businessID), zap.Error(err))
		}

		// Build final list: existing IDs + get-or-create from names
		seen := make(map[string]bool)
		var finalIDs []string
		if req.CategoryIDs != nil {
			for _, id := range req.CategoryIDs {
				if id != "" && !seen[id] {
					seen[id] = true
					finalIDs = append(finalIDs, id)
				}
			}
		}
		if req.CategoryNames != nil {
			for _, name := range req.CategoryNames {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				id, err := s.businessRepo.GetOrCreateCategoryByName(ctx, name)
				if err != nil {
					s.logger.Warn("GetOrCreateCategoryByName failed", zap.String("name", name), zap.Error(err))
					continue
				}
				if !seen[id] {
					seen[id] = true
					finalIDs = append(finalIDs, id)
				}
			}
		}
		if len(finalIDs) > 0 {
			if err := s.businessRepo.AddCategories(ctx, businessID, finalIDs); err != nil {
				s.logger.Error("Failed to add business categories", zap.String("business_id", businessID), zap.Error(err))
			}
		}
	}

	s.logger.Info("Business updated", zap.String("business_id", businessID), zap.String("user_id", userID))

	// Return enriched business
	return s.GetBusiness(ctx, businessID, &userID)
}

// DeleteBusiness soft deletes a business profile
func (s *BusinessService) DeleteBusiness(ctx context.Context, businessID, userID string) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to delete this business", nil)
	}

	// Delete business
	if err := s.businessRepo.Delete(ctx, businessID); err != nil {
		s.logger.Error("Failed to delete business", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to delete business", err)
	}

	s.logger.Info("Business deleted", zap.String("business_id", businessID), zap.String("user_id", userID))
	return nil
}

// SetBusinessHours sets operating hours for a business
func (s *BusinessService) SetBusinessHours(ctx context.Context, businessID, userID string, req *models.SetBusinessHoursRequest) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Delete existing hours
	if err := s.businessRepo.DeleteHoursByBusinessID(ctx, businessID); err != nil {
		s.logger.Error("Failed to delete existing hours", zap.String("business_id", businessID), zap.Error(err))
	}

	// Add new hours
	now := time.Now()
	for _, hourReq := range req.Hours {
		hours := &models.BusinessHours{
			ID:                uuid.New().String(),
			BusinessProfileID: businessID,
			Day:               hourReq.Day,
			IsClosed:          hourReq.IsClosed,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		// Parse time strings if provided and not closed
		if !hourReq.IsClosed {
			if hourReq.OpenTime != "" {
				openTime, err := time.Parse("15:04", hourReq.OpenTime)
				if err == nil {
					hours.OpenTime = &openTime
				}
			}
			if hourReq.CloseTime != "" {
				closeTime, err := time.Parse("15:04", hourReq.CloseTime)
				if err == nil {
					hours.CloseTime = &closeTime
				}
			}
		}

		if err := s.businessRepo.UpsertHours(ctx, hours); err != nil {
			s.logger.Error("Failed to upsert business hours",
				zap.String("business_id", businessID),
				zap.String("day", hourReq.Day),
				zap.Error(err),
			)
			// Continue with other days
		}
	}

	s.logger.Info("Business hours set", zap.String("business_id", businessID))
	return nil
}

// UploadAvatar uploads a business avatar
func (s *BusinessService) UploadAvatar(ctx context.Context, businessID, userID, photoURL string) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Update avatar
	business.Avatar = &models.Photo{URL: photoURL}
	business.UpdatedAt = time.Now()

	if err := s.businessRepo.Update(ctx, business); err != nil {
		s.logger.Error("Failed to update business avatar", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to update avatar", err)
	}

	s.logger.Info("Business avatar uploaded", zap.String("business_id", businessID))
	return nil
}

// UploadCover uploads a business cover photo
func (s *BusinessService) UploadCover(ctx context.Context, businessID, userID, photoURL string) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Update cover
	business.Cover = &models.Photo{URL: photoURL}
	business.UpdatedAt = time.Now()

	if err := s.businessRepo.Update(ctx, business); err != nil {
		s.logger.Error("Failed to update business cover", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to update cover", err)
	}

	s.logger.Info("Business cover uploaded", zap.String("business_id", businessID))
	return nil
}

const maxBusinessGalleryImages = 10

// AddGalleryImage adds an image to business gallery (max 10 per business).
func (s *BusinessService) AddGalleryImage(ctx context.Context, businessID, userID, photoURL string) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Enforce gallery limit
	existing, err := s.businessRepo.GetAttachmentsByBusinessID(ctx, businessID)
	if err != nil {
		return utils.NewInternalError("Failed to get gallery", err)
	}
	if len(existing) >= maxBusinessGalleryImages {
		return utils.NewBadRequestError("Gallery limit reached (max 10 images)", nil)
	}

	// Add attachment
	now := time.Now()
	attachment := &models.BusinessAttachment{
		ID:                uuid.New().String(),
		BusinessProfileID: businessID,
		Photo:             models.Photo{URL: photoURL},
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.businessRepo.AddAttachment(ctx, attachment); err != nil {
		s.logger.Error("Failed to add gallery image", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to add gallery image", err)
	}

	s.logger.Info("Gallery image added", zap.String("business_id", businessID))
	return nil
}

// DeleteGalleryImage removes an image from business gallery
func (s *BusinessService) DeleteGalleryImage(ctx context.Context, businessID, userID, attachmentID string) error {
	// Get existing business
	business, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Check ownership
	if business.UserID != userID {
		return utils.NewUnauthorizedError("You don't have permission to update this business", nil)
	}

	// Delete attachment
	if err := s.businessRepo.DeleteAttachment(ctx, attachmentID); err != nil {
		s.logger.Error("Failed to delete gallery image", zap.String("attachment_id", attachmentID), zap.Error(err))
		return utils.NewInternalError("Failed to delete gallery image", err)
	}

	s.logger.Info("Gallery image deleted", zap.String("attachment_id", attachmentID))
	return nil
}

// FollowBusiness follows a business
func (s *BusinessService) FollowBusiness(ctx context.Context, businessID, userID string) error {
	// Check if business exists
	if _, err := s.businessRepo.GetByID(ctx, businessID); err != nil {
		return utils.NewNotFoundError("Business not found", err)
	}

	// Follow business
	if err := s.businessRepo.Follow(ctx, businessID, userID); err != nil {
		s.logger.Error("Failed to follow business", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to follow business", err)
	}

	s.logger.Info("Business followed", zap.String("business_id", businessID), zap.String("user_id", userID))
	return nil
}

// UnfollowBusiness unfollows a business
func (s *BusinessService) UnfollowBusiness(ctx context.Context, businessID, userID string) error {
	// Unfollow business
	if err := s.businessRepo.Unfollow(ctx, businessID, userID); err != nil {
		s.logger.Error("Failed to unfollow business", zap.String("business_id", businessID), zap.Error(err))
		return utils.NewInternalError("Failed to unfollow business", err)
	}

	s.logger.Info("Business unfollowed", zap.String("business_id", businessID), zap.String("user_id", userID))
	return nil
}

// ListBusinesses lists business profiles with filters
func (s *BusinessService) ListBusinesses(ctx context.Context, filter *models.BusinessListFilter, viewerID *string) ([]*models.BusinessResponse, error) {
	// Get businesses
	businesses, err := s.businessRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list businesses", zap.Error(err))
		return nil, utils.NewInternalError("Failed to list businesses", err)
	}

	// Enrich businesses
	var enrichedBusinesses []*models.BusinessResponse
	for _, business := range businesses {
		enriched, err := s.enrichBusiness(ctx, business, viewerID)
		if err != nil {
			s.logger.Warn("Failed to enrich business", zap.String("business_id", business.ID), zap.Error(err))
			continue
		}
		enrichedBusinesses = append(enrichedBusinesses, enriched)
	}

	return enrichedBusinesses, nil
}

// GetAllCategories gets all business categories, optionally filtered by search (name).
func (s *BusinessService) GetAllCategories(ctx context.Context, search *string) ([]*models.BusinessCategory, error) {
	categories, err := s.businessRepo.GetAllCategories(ctx, search)
	if err != nil {
		s.logger.Error("Failed to get business categories", zap.Error(err))
		return nil, utils.NewInternalError("Failed to get categories", err)
	}
	if categories == nil {
		categories = []*models.BusinessCategory{}
	}
	return categories, nil
}

// GetBusinessGallery returns all gallery attachments for a business (separate from profile).
func (s *BusinessService) GetBusinessGallery(ctx context.Context, businessID string) ([]*models.GalleryItem, error) {
	attachments, err := s.businessRepo.GetAttachmentsByBusinessID(ctx, businessID)
	if err != nil {
		s.logger.Error("Failed to get business gallery", zap.String("business_id", businessID), zap.Error(err))
		return nil, utils.NewInternalError("Failed to get gallery", err)
	}
	if len(attachments) == 0 {
		return []*models.GalleryItem{}, nil
	}
	out := make([]*models.GalleryItem, len(attachments))
	for i, att := range attachments {
		out[i] = &models.GalleryItem{ID: att.ID, Photo: att.Photo}
	}
	return out, nil
}

// enrichBusiness enriches a business with categories, hours, and following status (gallery is separate endpoint).
func (s *BusinessService) enrichBusiness(ctx context.Context, business *models.BusinessProfile, viewerID *string) (*models.BusinessResponse, error) {
	response := &models.BusinessResponse{
		ID:             business.ID,
		UserID:         business.UserID,
		Name:           business.Name,
		LicenseNo:      business.LicenseNo,
		Description:    business.Description,
		Address:        business.Address,
		PhoneNumber:    business.PhoneNumber,
		Email:          business.Email,
		Website:        business.Website,
		Avatar:         business.Avatar,
		Cover:          business.Cover,
		Status:         business.Status,
		AdditionalInfo: business.AdditionalInfo,
		Province:       business.Province,
		District:       business.District,
		Neighborhood:   business.Neighborhood,
		ShowLocation:   business.ShowLocation,
		TotalViews:     business.TotalViews,
		TotalFollow:    business.TotalFollow,
		CreatedAt:      business.CreatedAt,
		UpdatedAt:      business.UpdatedAt,
	}

	// Add location info
	if business.AddressLocation != nil && business.AddressLocation.Valid {
		response.Location = &models.LocationInfo{
			Latitude:     &business.AddressLocation.P.Y,
			Longitude:    &business.AddressLocation.P.X,
			Country:      business.Country,
			Province:     business.Province,
			District:     business.District,
			Neighborhood: business.Neighborhood,
		}
	}

	// Get categories (always set so API returns "categories" key, never null)
	response.Categories = []models.BusinessCategory{}
	categories, err := s.businessRepo.GetCategoriesByBusinessID(ctx, business.ID)
	if err == nil && len(categories) > 0 {
		response.Categories = make([]models.BusinessCategory, len(categories))
		for i, cat := range categories {
			response.Categories[i] = *cat
		}
	}

	// Get business hours
	hours, err := s.businessRepo.GetHoursByBusinessID(ctx, business.ID)
	if err == nil && len(hours) > 0 {
		var hoursResponse []models.BusinessHoursResponse
		for _, h := range hours {
			hourResp := models.BusinessHoursResponse{
				Day:      h.Day,
				IsClosed: h.IsClosed,
			}
			if h.OpenTime != nil {
				timeStr := fmt.Sprintf("%02d:%02d", h.OpenTime.Hour(), h.OpenTime.Minute())
				hourResp.OpenTime = &timeStr
			}
			if h.CloseTime != nil {
				timeStr := fmt.Sprintf("%02d:%02d", h.CloseTime.Hour(), h.CloseTime.Minute())
				hourResp.CloseTime = &timeStr
			}
			hoursResponse = append(hoursResponse, hourResp)
		}
		response.Hours = hoursResponse
	}

	// Get following status if viewer is authenticated
	if viewerID != nil && *viewerID != "" {
		isFollowing, err := s.businessRepo.IsFollowing(ctx, business.ID, *viewerID)
		if err == nil {
			response.IsFollowing = isFollowing
		}
	}

	return response, nil
}

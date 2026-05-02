package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// BusinessReviewHandler exposes the review endpoints under
// /api/v1/businesses/:business_id/reviews.
type BusinessReviewHandler struct {
	service   *services.BusinessReviewService
	userRepo  repositories.UserRepository
	validator *utils.Validator
	logger    *zap.Logger
}

// NewBusinessReviewHandler wires the handler.
func NewBusinessReviewHandler(
	service *services.BusinessReviewService,
	userRepo repositories.UserRepository,
	validator *utils.Validator,
	logger *zap.Logger,
) *BusinessReviewHandler {
	return &BusinessReviewHandler{
		service:   service,
		userRepo:  userRepo,
		validator: validator,
		logger:    logger,
	}
}

func (h *BusinessReviewHandler) sendErr(c *gin.Context, err error) {
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}
	h.logger.Error("Unhandled error in business review handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}

func (h *BusinessReviewHandler) currentUser(c *gin.Context) (string, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return "", false
	}
	return v.(string), true
}

func (h *BusinessReviewHandler) isAdmin(c *gin.Context) bool {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		return false
	}
	userID := userIDVal.(string)
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		return false
	}
	return user.Role == models.RoleAdmin || user.Role == models.RoleSuperAdmin || user.Role == models.RoleModerator
}

// SubmitReview creates or replaces the caller's review for a business.
// @Tags         business-reviews
// @Security     BearerAuth
// @Param        business_id path string true "Business profile id"
// @Param        request body models.CreateBusinessReviewRequest true "Review"
// @Success      201 {object} utils.Response{data=models.BusinessReview}
// @Router       /businesses/{business_id}/reviews [post]
func (h *BusinessReviewHandler) SubmitReview(c *gin.Context) {
	userID, ok := h.currentUser(c)
	if !ok {
		return
	}
	businessID := c.Param("business_id")
	if businessID == "" {
		utils.SendError(c, http.StatusBadRequest, "business_id is required", utils.ErrBadRequest)
		return
	}

	var req models.CreateBusinessReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	review, err := h.service.Submit(c.Request.Context(), businessID, userID, &req)
	if err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Review submitted", review)
}

// UpdateReview edits the caller's existing review.
// @Tags         business-reviews
// @Security     BearerAuth
// @Param        business_id path string true "Business profile id"
// @Param        review_id path string true "Review id"
// @Param        request body models.UpdateBusinessReviewRequest true "Update"
// @Success      200 {object} utils.Response{data=models.BusinessReview}
// @Router       /businesses/{business_id}/reviews/{review_id} [put]
func (h *BusinessReviewHandler) UpdateReview(c *gin.Context) {
	userID, ok := h.currentUser(c)
	if !ok {
		return
	}
	reviewID := c.Param("review_id")
	if reviewID == "" {
		utils.SendError(c, http.StatusBadRequest, "review_id is required", utils.ErrBadRequest)
		return
	}

	var req models.UpdateBusinessReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	review, err := h.service.Update(c.Request.Context(), reviewID, userID, &req)
	if err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Review updated", review)
}

// DeleteReview removes the caller's review (or any review when admin).
// @Tags         business-reviews
// @Security     BearerAuth
// @Param        business_id path string true "Business profile id"
// @Param        review_id path string true "Review id"
// @Success      204 {object} utils.Response
// @Router       /businesses/{business_id}/reviews/{review_id} [delete]
func (h *BusinessReviewHandler) DeleteReview(c *gin.Context) {
	userID, ok := h.currentUser(c)
	if !ok {
		return
	}
	reviewID := c.Param("review_id")
	if reviewID == "" {
		utils.SendError(c, http.StatusBadRequest, "review_id is required", utils.ErrBadRequest)
		return
	}
	if err := h.service.Delete(c.Request.Context(), reviewID, userID, h.isAdmin(c)); err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Review deleted", nil)
}

// ListReviews returns paginated reviews for a business (public).
// @Tags         business-reviews
// @Param        business_id path string true "Business profile id"
// @Param        limit query int false "Page size (default 20, max 100)"
// @Param        offset query int false "Offset (default 0)"
// @Success      200 {object} utils.Response
// @Router       /businesses/{business_id}/reviews [get]
func (h *BusinessReviewHandler) ListReviews(c *gin.Context) {
	businessID := c.Param("business_id")
	if businessID == "" {
		utils.SendError(c, http.StatusBadRequest, "business_id is required", utils.ErrBadRequest)
		return
	}

	limit := 20
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	includeHidden := h.isAdmin(c)

	reviews, total, err := h.service.List(c.Request.Context(), businessID, includeHidden, limit, offset)
	if err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Reviews", gin.H{
		"items":  reviews,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMyReview returns the caller's review for a business, or null.
// @Tags         business-reviews
// @Security     BearerAuth
// @Param        business_id path string true "Business profile id"
// @Success      200 {object} utils.Response
// @Router       /businesses/{business_id}/reviews/me [get]
func (h *BusinessReviewHandler) GetMyReview(c *gin.Context) {
	userID, ok := h.currentUser(c)
	if !ok {
		return
	}
	businessID := c.Param("business_id")
	review, err := h.service.GetMyReview(c.Request.Context(), businessID, userID)
	if err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Review", review)
}

// GetStats returns the aggregate stats for the summary card.
// @Tags         business-reviews
// @Param        business_id path string true "Business profile id"
// @Success      200 {object} utils.Response{data=models.BusinessReviewStats}
// @Router       /businesses/{business_id}/reviews/stats [get]
func (h *BusinessReviewHandler) GetStats(c *gin.Context) {
	businessID := c.Param("business_id")
	stats, err := h.service.Stats(c.Request.Context(), businessID)
	if err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Stats", stats)
}

// SetHidden toggles moderation visibility (admin only).
// @Tags         admin
// @Security     BearerAuth
// @Param        review_id path string true "Review id"
// @Param        hidden query bool true "Hidden flag"
// @Success      200 {object} utils.Response
// @Router       /admin/business-reviews/{review_id}/hidden [patch]
func (h *BusinessReviewHandler) SetHidden(c *gin.Context) {
	reviewID := c.Param("review_id")
	hiddenStr := c.Query("hidden")
	hidden, err := strconv.ParseBool(hiddenStr)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, "hidden must be true|false", utils.ErrBadRequest)
		return
	}
	if err := h.service.SetHidden(c.Request.Context(), reviewID, hidden); err != nil {
		h.sendErr(c, err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Updated", nil)
}

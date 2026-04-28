package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
)

// MonetizationHandler exposes the admin-facing /admin/ads /admin/credits and
// /admin/boosts endpoints. Routes are registered in cmd/server/main.go.
type MonetizationHandler struct {
	service   *services.MonetizationService
	validator *utils.Validator
	logger    *zap.Logger
}

func NewMonetizationHandler(
	service *services.MonetizationService,
	validator *utils.Validator,
	logger *zap.Logger,
) *MonetizationHandler {
	return &MonetizationHandler{service: service, validator: validator, logger: logger}
}

// ─── Ads ─────────────────────────────────────────────────────────────────────

// ListAds godoc
// @Router /admin/ads [get]
func (h *MonetizationHandler) ListAds(c *gin.Context) {
	status := c.Query("status")
	page := atoiOr(c.Query("page"), 1)
	limit := atoiOr(c.Query("limit"), 20)

	ads, total, err := h.service.ListAds(c.Request.Context(), status, page, limit)
	if err != nil {
		h.logger.Error("list ads", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to list ads", err)
		return
	}
	if ads == nil {
		ads = []*models.Ad{}
	}
	utils.SendSuccess(c, http.StatusOK, "Ads retrieved", paginated(ads, total, page, limit))
}

func (h *MonetizationHandler) GetAd(c *gin.Context) {
	id := c.Param("ad_id")
	ad, err := h.service.GetAd(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrAdNotFound) {
			utils.SendError(c, http.StatusNotFound, "Ad not found", err)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Failed to load ad", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ad detail", ad)
}

func (h *MonetizationHandler) ApproveAd(c *gin.Context) {
	id := c.Param("ad_id")
	var req models.AdReviewRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	ad, err := h.service.Approve(c.Request.Context(), id, adminUserID(c), &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAdNotFound):
			utils.SendError(c, http.StatusNotFound, "Ad not found", err)
		case errors.Is(err, services.ErrInvalidAdStatus):
			utils.SendError(c, http.StatusBadRequest, "Invalid status transition", err)
		default:
			utils.SendError(c, http.StatusInternalServerError, "Failed to approve", err)
		}
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ad approved", ad)
}

func (h *MonetizationHandler) RejectAd(c *gin.Context) {
	id := c.Param("ad_id")
	var req models.AdReviewRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	ad, err := h.service.Reject(c.Request.Context(), id, adminUserID(c), &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAdNotFound):
			utils.SendError(c, http.StatusNotFound, "Ad not found", err)
		case errors.Is(err, services.ErrInvalidAdStatus):
			utils.SendError(c, http.StatusBadRequest, "Invalid status transition", err)
		default:
			utils.SendError(c, http.StatusInternalServerError, "Failed to reject", err)
		}
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ad rejected", ad)
}

func (h *MonetizationHandler) DeleteAd(c *gin.Context) {
	id := c.Param("ad_id")
	if err := h.service.DeleteAd(c.Request.Context(), id); err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to delete", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Ad deleted", nil)
}

// ─── Credits ─────────────────────────────────────────────────────────────────

func (h *MonetizationHandler) ListBalances(c *gin.Context) {
	search := c.Query("search")
	page := atoiOr(c.Query("page"), 1)
	limit := atoiOr(c.Query("limit"), 20)
	balances, total, err := h.service.ListBalances(c.Request.Context(), search, page, limit)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to list balances", err)
		return
	}
	if balances == nil {
		balances = []*models.CreditBalance{}
	}
	utils.SendSuccess(c, http.StatusOK, "Credit balances", paginated(balances, total, page, limit))
}

func (h *MonetizationHandler) GetUserCredits(c *gin.Context) {
	userID := c.Param("user_id")
	detail, err := h.service.GetUserCredits(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, http.StatusNotFound, "User not found", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User credits", detail)
}

func (h *MonetizationHandler) AdjustUserCredits(c *gin.Context) {
	userID := c.Param("user_id")
	var req models.AdjustCreditsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	balance, err := h.service.AdjustCredits(c.Request.Context(), userID, &req, adminUserID(c))
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Credits adjusted", balance)
}

// ─── Boosts ──────────────────────────────────────────────────────────────────

func (h *MonetizationHandler) ListBoosts(c *gin.Context) {
	status := c.Query("status")
	page := atoiOr(c.Query("page"), 1)
	limit := atoiOr(c.Query("limit"), 20)
	boosts, total, err := h.service.ListBoosts(c.Request.Context(), status, page, limit)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to list boosts", err)
		return
	}
	if boosts == nil {
		boosts = []*models.Boost{}
	}
	utils.SendSuccess(c, http.StatusOK, "Boosts retrieved", paginated(boosts, total, page, limit))
}

func (h *MonetizationHandler) CancelBoost(c *gin.Context) {
	id := c.Param("boost_id")
	var req models.CancelBoostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	boost, err := h.service.CancelBoost(c.Request.Context(), id, adminUserID(c), req.Reason)
	if err != nil {
		if errors.Is(err, services.ErrBoostNotFound) {
			utils.SendError(c, http.StatusNotFound, "Boost not found or already inactive", err)
			return
		}
		utils.SendError(c, http.StatusInternalServerError, "Failed to cancel", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Boost cancelled", boost)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func adminUserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// paginated produces the standard `{ items, total_count, page, limit, total_pages }`
// envelope used across the admin panel list endpoints.
func paginated[T any](items []T, total, page, limit int) gin.H {
	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return gin.H{
		"items":       items,
		"total_count": total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	}
}

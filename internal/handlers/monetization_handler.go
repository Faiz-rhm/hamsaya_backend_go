package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
)

// adImpressionDedupeTTL caps how often a single (ad, IP) pair can record an
// impression — protects advertiser metrics from script abuse.
const adImpressionDedupeTTL = 30 * time.Minute

// adClickDedupeTTL — clicks are rarer per user; longer window.
const adClickDedupeTTL = 1 * time.Hour

// MonetizationHandler exposes the admin-facing /admin/ads /admin/credits and
// /admin/boosts endpoints. Routes are registered in cmd/server/main.go.
type MonetizationHandler struct {
	service   *services.MonetizationService
	storage   *services.StorageService
	validator *utils.Validator
	logger    *zap.Logger
	redis     *redis.Client
}

func NewMonetizationHandler(
	service *services.MonetizationService,
	storage *services.StorageService,
	validator *utils.Validator,
	logger *zap.Logger,
	redisClient *redis.Client,
) *MonetizationHandler {
	return &MonetizationHandler{
		service:   service,
		storage:   storage,
		validator: validator,
		logger:    logger,
		redis:     redisClient,
	}
}

// ─── Ads ─────────────────────────────────────────────────────────────────────

// ─── Public (mobile-facing) ─────────────────────────────────────────────────

// ListActiveAdsPublic returns currently-live ads. No auth required so the
// mobile feed can fetch even before the user signs in.
//
// @Router /ads/active [get]
func (h *MonetizationHandler) ListActiveAdsPublic(c *gin.Context) {
	limit := atoiOr(c.Query("limit"), 10)
	// User context for targeting. Mobile passes its own province + locale so
	// the server can match against ads.target_provinces / target_languages.
	// Empty values disable targeting on that dimension.
	province := c.Query("province")
	language := c.Query("language")
	ads, err := h.service.ListActiveAds(c.Request.Context(), limit, province, language)
	if err != nil {
		h.logger.Error("public active ads", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to load ads", err)
		return
	}
	if ads == nil {
		ads = []*models.Ad{}
	}
	utils.SendSuccess(c, http.StatusOK, "Active ads", gin.H{"items": ads})
}

// RecordAdImpression — public, fire-and-forget impression tracker called by
// mobile when an ad enters the visible viewport. Per-(ad,IP) Redis SETEX
// dedupe so repeated requests within the TTL collapse to one count.
//
// @Router /ads/{ad_id}/impression [post]
func (h *MonetizationHandler) RecordAdImpression(c *gin.Context) {
	id := c.Param("ad_id")
	if !h.shouldRecordAdEvent(c, "imp", id, adImpressionDedupeTTL) {
		utils.SendSuccess(c, http.StatusOK, "ok", nil)
		return
	}
	if err := h.service.RecordImpression(c.Request.Context(), id); err != nil {
		h.logger.Warn("ad impression", zap.Error(err))
	}
	utils.SendSuccess(c, http.StatusOK, "ok", nil)
}

// RecordAdClick — public, called by mobile before opening the target URL.
// Same per-(ad,IP) dedupe as impressions, longer TTL.
//
// @Router /ads/{ad_id}/click [post]
func (h *MonetizationHandler) RecordAdClick(c *gin.Context) {
	id := c.Param("ad_id")
	if !h.shouldRecordAdEvent(c, "click", id, adClickDedupeTTL) {
		utils.SendSuccess(c, http.StatusOK, "ok", nil)
		return
	}
	if err := h.service.RecordClick(c.Request.Context(), id); err != nil {
		h.logger.Warn("ad click", zap.Error(err))
	}
	utils.SendSuccess(c, http.StatusOK, "ok", nil)
}

// shouldRecordAdEvent returns true on the first request from this IP for this
// ad within `ttl`. Subsequent requests inside the window are silently
// swallowed without incrementing counters. Fails open (records the event) on
// Redis errors — the rate-limit middleware already throttles raw request
// volume, so a Redis outage shouldn't block legitimate ad analytics.
func (h *MonetizationHandler) shouldRecordAdEvent(c *gin.Context, kind, adID string, ttl time.Duration) bool {
	if h.redis == nil {
		return true
	}
	key := fmt.Sprintf("ad-dedupe:%s:%s:%s", kind, adID, c.ClientIP())
	ok, err := h.redis.SetNX(c.Request.Context(), key, "1", ttl).Result()
	if err != nil {
		h.logger.Warn("ad dedupe SETNX failed", zap.Error(err))
		return true
	}
	return ok
}

// CreateAd accepts a multipart form: required `image` file, plus form fields
// matching AdCreateRequest (advertiser_id, title, body, target_url, start_at,
// end_at, auto_approve).
//
// @Router /admin/ads [post]
func (h *MonetizationHandler) CreateAd(c *gin.Context) {
	var req models.AdCreateRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid form data", err)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Default attribution: ad belongs to the admin who created it unless the
	// admin explicitly named a different advertiser_id.
	if req.AdvertiserID == "" {
		req.AdvertiserID = adminUserID(c)
	}
	if req.AdvertiserID == "" {
		utils.SendError(c, http.StatusBadRequest, "advertiser_id required", utils.ErrValidation)
		return
	}

	imageURL := ""
	if file, header, err := c.Request.FormFile("image"); err == nil {
		defer func() { _ = file.Close() }()
		if !utils.EnforceUploadSize(c, header.Size, utils.MaxImageUploadBytes) {
			return
		}
		photo, uErr := h.storage.UploadImage(c.Request.Context(), file, header, services.ImageTypeAd)
		if uErr != nil {
			h.logger.Error("ad image upload", zap.Error(uErr))
			utils.SendError(c, http.StatusInternalServerError, "Failed to upload image", uErr)
			return
		}
		imageURL = photo.URL
	}

	ad, err := h.service.CreateAd(c.Request.Context(), &req, imageURL)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Ad created", ad)
}

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

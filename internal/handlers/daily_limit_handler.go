package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
)

// DailyLimitHandler exposes daily-post-limit usage for users and admin
// CRUD-style management of the limit rows themselves.
type DailyLimitHandler struct {
	service   *services.DailyLimitService
	userRepo  repositories.UserRepository
	validator *utils.Validator
	logger    *zap.Logger
}

func NewDailyLimitHandler(
	service *services.DailyLimitService,
	userRepo repositories.UserRepository,
	validator *utils.Validator,
	logger *zap.Logger,
) *DailyLimitHandler {
	return &DailyLimitHandler{
		service:   service,
		userRepo:  userRepo,
		validator: validator,
		logger:    logger,
	}
}

// GetMyDailyLimits godoc
// @Summary    Per-post-type daily creation usage for the authenticated user
// @Description Returns used / limit / remaining / resets_at for each post type.
//
//	The mobile app calls this to render quota headers and disable
//	create buttons when the limit is hit. Optional ?on_business=true
//	flag applies the BusinessMultiplier so the response reflects the
//	cap when the user is acting as a business.
//
// @Tags posts
// @Produce json
// @Param on_business query bool false "Apply business multiplier to limits"
// @Success 200 {object} utils.Response{data=[]models.DailyLimitUsage}
// @Router  /posts/daily-limits [get]
func (h *DailyLimitHandler) GetMyDailyLimits(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}
	userID := userIDVal.(string)

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	role := models.RoleUser
	if err == nil && user != nil {
		role = user.Role
	}

	onBusiness := c.Query("on_business") == "true"

	usage, err := h.service.GetUsage(c.Request.Context(), userID, role, onBusiness)
	if err != nil {
		h.logger.Error("daily limit usage", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to fetch usage", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Daily limit usage", usage)
}

// ─── Admin endpoints ──────────────────────────────────────────────────────────

// AdminListLimits godoc
// @Summary List daily post limit rows (admin)
// @Tags    admin,posts
// @Produce json
// @Success 200 {object} utils.Response{data=[]models.DailyPostLimit}
// @Router  /admin/daily-limits [get]
func (h *DailyLimitHandler) AdminListLimits(c *gin.Context) {
	limits, err := h.service.ListLimits(c.Request.Context())
	if err != nil {
		h.logger.Error("admin list limits", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to list limits", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Daily post limits", limits)
}

// AdminUpdateLimit godoc
// @Summary Update a single daily post limit (admin)
// @Description Update the user_limit and/or business_multiplier for a given
//
//	post type. Both fields optional; missing fields keep their
//	current value. Cache is busted so changes apply within seconds.
//
// @Tags    admin,posts
// @Accept  json
// @Produce json
// @Param   post_type path     string                              true "FEED|EVENT|SELL|PULL"
// @Param   request   body     models.UpdateDailyPostLimitRequest  true "Fields to update"
// @Success 200       {object} utils.Response{data=models.DailyPostLimit}
// @Failure 400       {object} utils.Response
// @Failure 404       {object} utils.Response
// @Router  /admin/daily-limits/{post_type} [put]
func (h *DailyLimitHandler) AdminUpdateLimit(c *gin.Context) {
	postType := c.Param("post_type")
	if postType == "" {
		utils.SendError(c, http.StatusBadRequest, "post_type is required", utils.ErrBadRequest)
		return
	}

	var req models.UpdateDailyPostLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}
	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	adminUserID := ""
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			adminUserID = s
		}
	}

	limit, err := h.service.UpdateLimit(c.Request.Context(), postType, &req, adminUserID)
	if err != nil {
		if appErr, ok := err.(*utils.AppError); ok {
			utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
			return
		}
		h.logger.Error("admin update limit", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to update limit", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Daily post limit updated", limit)
}

// ─── Per-user overrides ──────────────────────────────────────────────────────

// AdminListAllOverrides godoc
// @Router /admin/daily-limits/overrides [get]
func (h *DailyLimitHandler) AdminListAllOverrides(c *gin.Context) {
	rows, err := h.service.ListUserOverrides(c.Request.Context())
	if err != nil {
		h.logger.Error("list daily limit overrides", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"overrides": rows})
}

// AdminListOverridesForUser godoc
// @Router /admin/users/{user_id}/daily-limits [get]
func (h *DailyLimitHandler) AdminListOverridesForUser(c *gin.Context) {
	userID := c.Param("user_id")
	rows, err := h.service.ListUserOverridesFor(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("list user daily limit overrides", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"overrides": rows})
}

type setDailyLimitOverrideBody struct {
	OverrideLimit *int   `json:"override_limit"`
	Unlimited     bool   `json:"unlimited"`
	Reason        string `json:"reason"`
}

// AdminSetOverrideForUser godoc
// @Router /admin/users/{user_id}/daily-limits/{post_type} [put]
func (h *DailyLimitHandler) AdminSetOverrideForUser(c *gin.Context) {
	userID := c.Param("user_id")
	postType := c.Param("post_type")
	if userID == "" || postType == "" {
		utils.SendError(c, http.StatusBadRequest, "user_id and post_type required", utils.ErrValidation)
		return
	}

	var body setDailyLimitOverrideBody
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", err)
		return
	}

	adminID := ""
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			adminID = s
		}
	}

	if err := h.service.SetUserOverride(c.Request.Context(), userID, postType,
		body.OverrideLimit, body.Unlimited, body.Reason, adminID,
	); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Override set", nil)
}

// AdminDeleteOverrideForUser godoc
// @Router /admin/users/{user_id}/daily-limits/{post_type} [delete]
func (h *DailyLimitHandler) AdminDeleteOverrideForUser(c *gin.Context) {
	userID := c.Param("user_id")
	postType := c.Param("post_type")
	if err := h.service.DeleteUserOverride(c.Request.Context(), userID, postType); err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Override removed", nil)
}

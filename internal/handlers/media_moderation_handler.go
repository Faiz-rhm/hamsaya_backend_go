package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// MediaModerationHandler exposes the admin media-review queue.
type MediaModerationHandler struct {
	svc          *services.MediaModerationService
	adminService *services.AdminService
	logger       *zap.Logger
}

func NewMediaModerationHandler(svc *services.MediaModerationService, adminService *services.AdminService, logger *zap.Logger) *MediaModerationHandler {
	return &MediaModerationHandler{svc: svc, adminService: adminService, logger: logger}
}

// List godoc
// @Router /admin/media-moderation [get]
func (h *MediaModerationHandler) List(c *gin.Context) {
	status := c.Query("status")
	rows, err := h.svc.List(c.Request.Context(), status, 100)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Query failed", err)
		return
	}
	counts, _ := h.svc.Counts(c.Request.Context())
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"items": rows, "counts": counts})
}

type reviewMediaBody struct {
	Notes string `json:"notes"`
}

// Approve godoc
// @Router /admin/media-moderation/{attachment_id}/approve [post]
func (h *MediaModerationHandler) Approve(c *gin.Context) {
	id := c.Param("attachment_id")
	var body reviewMediaBody
	_ = c.ShouldBindJSON(&body)
	adminID, _ := middleware.GetUserID(c)
	if err := h.svc.Approve(c.Request.Context(), id, adminID, body.Notes); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "approve_media", "attachment", id,
		map[string]interface{}{"notes": body.Notes}, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Approved", nil)
}

// Reject godoc
// @Router /admin/media-moderation/{attachment_id}/reject [post]
func (h *MediaModerationHandler) Reject(c *gin.Context) {
	id := c.Param("attachment_id")
	var body reviewMediaBody
	_ = c.ShouldBindJSON(&body)
	adminID, _ := middleware.GetUserID(c)
	if err := h.svc.Reject(c.Request.Context(), id, adminID, body.Notes); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "reject_media", "attachment", id,
		map[string]interface{}{"notes": body.Notes}, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Rejected + attachment deleted", nil)
}

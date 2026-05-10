package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// DeletionRequestHandler exposes admin endpoints for the GDPR-style
// account deletion review queue.
type DeletionRequestHandler struct {
	svc          *services.DeletionRequestService
	adminService *services.AdminService
	logger       *zap.Logger
}

func NewDeletionRequestHandler(svc *services.DeletionRequestService, adminService *services.AdminService, logger *zap.Logger) *DeletionRequestHandler {
	return &DeletionRequestHandler{svc: svc, adminService: adminService, logger: logger}
}

type createDeletionRequestBody struct {
	UserID string `json:"user_id" binding:"required,uuid"`
	Reason string `json:"reason"`
}

// Create godoc
// @Router /admin/deletion-requests [post]
func (h *DeletionRequestHandler) Create(c *gin.Context) {
	var body createDeletionRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", err)
		return
	}
	adminID, _ := middleware.GetUserID(c)
	id, err := h.svc.Create(c.Request.Context(), body.UserID, body.Reason, c.ClientIP())
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to create request", err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "create_deletion_request",
		"deletion_request", id.String(), map[string]interface{}{"user_id": body.UserID, "reason": body.Reason}, c.ClientIP())
	utils.SendSuccess(c, http.StatusCreated, "Created", gin.H{"id": id.String()})
}

// List godoc
// @Router /admin/deletion-requests [get]
func (h *DeletionRequestHandler) List(c *gin.Context) {
	status := c.Query("status")
	rows, err := h.svc.List(c.Request.Context(), status, 100)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Query failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"requests": rows})
}

type reviewBody struct {
	Notes string `json:"notes"`
}

// Approve godoc
// @Router /admin/deletion-requests/{id}/approve [post]
func (h *DeletionRequestHandler) Approve(c *gin.Context) {
	id := c.Param("id")
	var body reviewBody
	_ = c.ShouldBindJSON(&body)
	adminID, _ := middleware.GetUserID(c)

	if err := h.svc.Approve(c.Request.Context(), id, adminID, body.Notes); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "approve_deletion_request",
		"deletion_request", id, map[string]interface{}{"notes": body.Notes}, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Approved + user deleted", nil)
}

// Reject godoc
// @Router /admin/deletion-requests/{id}/reject [post]
func (h *DeletionRequestHandler) Reject(c *gin.Context) {
	id := c.Param("id")
	var body reviewBody
	_ = c.ShouldBindJSON(&body)
	adminID, _ := middleware.GetUserID(c)

	if err := h.svc.Reject(c.Request.Context(), id, adminID, body.Notes); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "reject_deletion_request",
		"deletion_request", id, map[string]interface{}{"notes": body.Notes}, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Rejected", nil)
}

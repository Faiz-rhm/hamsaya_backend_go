package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/middleware"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// AutomodHandler exposes admin CRUD on automod_rules.
type AutomodHandler struct {
	svc          *services.AutomodService
	adminService *services.AdminService
	logger       *zap.Logger
}

func NewAutomodHandler(svc *services.AutomodService, adminService *services.AdminService, logger *zap.Logger) *AutomodHandler {
	return &AutomodHandler{svc: svc, adminService: adminService, logger: logger}
}

// List godoc
// @Router /admin/automod/rules [get]
func (h *AutomodHandler) List(c *gin.Context) {
	rows, err := h.svc.List(c.Request.Context())
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Query failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"rules": rows})
}

type automodRuleBody struct {
	Pattern     string `json:"pattern" binding:"required"`
	IsRegex     bool   `json:"is_regex"`
	Action      string `json:"action" binding:"required"`
	Severity    string `json:"severity" binding:"required"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// Create godoc
// @Router /admin/automod/rules [post]
func (h *AutomodHandler) Create(c *gin.Context) {
	var b automodRuleBody
	if err := c.ShouldBindJSON(&b); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", err)
		return
	}
	adminID, _ := middleware.GetUserID(c)
	id, err := h.svc.Create(c.Request.Context(), b.Pattern, b.IsRegex, b.Action, b.Severity, b.Description, adminID)
	if err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "create_automod_rule", "automod_rule", id.String(),
		map[string]interface{}{"pattern": b.Pattern, "action": b.Action, "severity": b.Severity}, c.ClientIP())
	utils.SendSuccess(c, http.StatusCreated, "Created", gin.H{"id": id.String()})
}

// Update godoc
// @Router /admin/automod/rules/{id} [put]
func (h *AutomodHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var b automodRuleBody
	if err := c.ShouldBindJSON(&b); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", err)
		return
	}
	adminID, _ := middleware.GetUserID(c)
	if err := h.svc.Update(c.Request.Context(), id, b.Pattern, b.IsRegex, b.Action, b.Severity, b.Description, b.Enabled); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "update_automod_rule", "automod_rule", id,
		map[string]interface{}{"pattern": b.Pattern, "enabled": b.Enabled}, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Updated", nil)
}

// Delete godoc
// @Router /admin/automod/rules/{id} [delete]
func (h *AutomodHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	adminID, _ := middleware.GetUserID(c)
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}
	_ = h.adminService.LogAuditAction(c.Request.Context(), adminID, "delete_automod_rule", "automod_rule", id, nil, c.ClientIP())
	utils.SendSuccess(c, http.StatusOK, "Deleted", nil)
}

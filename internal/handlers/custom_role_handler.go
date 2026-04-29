package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
)

// CustomRoleHandler exposes admin CRUD for named permission roles and the
// assign/unassign user endpoint. All routes sit under /admin/custom-roles and
// require at minimum super_admin.
type CustomRoleHandler struct {
	repo   repositories.CustomRoleRepository
	logger *zap.Logger
}

func NewCustomRoleHandler(repo repositories.CustomRoleRepository, logger *zap.Logger) *CustomRoleHandler {
	return &CustomRoleHandler{repo: repo, logger: logger}
}

func (h *CustomRoleHandler) List(c *gin.Context) {
	roles, err := h.repo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("list custom roles", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to list roles", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Custom roles", roles)
}

func (h *CustomRoleHandler) Get(c *gin.Context) {
	id := c.Param("role_id")
	role, err := h.repo.Get(c.Request.Context(), id)
	if err != nil || role == nil {
		utils.SendError(c, http.StatusNotFound, "Role not found", nil)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Custom role", role)
}

func (h *CustomRoleHandler) Create(c *gin.Context) {
	var req models.CreateCustomRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", utils.ErrInvalidJSON)
		return
	}
	role, err := h.repo.Create(c.Request.Context(), &req, adminUserID(c))
	if err != nil {
		h.logger.Error("create custom role", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to create role", err)
		return
	}
	utils.SendSuccess(c, http.StatusCreated, "Role created", role)
}

func (h *CustomRoleHandler) Update(c *gin.Context) {
	id := c.Param("role_id")
	var req models.UpdateCustomRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", utils.ErrInvalidJSON)
		return
	}
	role, err := h.repo.Update(c.Request.Context(), id, &req, adminUserID(c))
	if err != nil || role == nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to update role", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Role updated", role)
}

func (h *CustomRoleHandler) Delete(c *gin.Context) {
	id := c.Param("role_id")
	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to delete role", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Role deleted", nil)
}

func (h *CustomRoleHandler) ListRoleUsers(c *gin.Context) {
	id := c.Param("role_id")
	users, err := h.repo.ListUsers(c.Request.Context(), id)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to list users", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Users with role", users)
}

// Assign sets or clears the custom_role_id for a user. POST with
// { "custom_role_id": "uuid" } assigns. POST with { "custom_role_id": null }
// or omitting the field clears.
func (h *CustomRoleHandler) Assign(c *gin.Context) {
	userID := c.Param("user_id")
	var req models.AssignCustomRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", utils.ErrInvalidJSON)
		return
	}
	if err := h.repo.Assign(c.Request.Context(), userID, req.CustomRoleID); err != nil {
		h.logger.Error("assign custom role", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to assign role", err)
		return
	}
	msg := "Custom role assigned"
	if req.CustomRoleID == nil || *req.CustomRoleID == "" {
		msg = "Custom role cleared"
	}
	utils.SendSuccess(c, http.StatusOK, msg, nil)
}

// GetUserCustomRole returns the custom role assigned to an admin user.
func (h *CustomRoleHandler) GetUserCustomRole(c *gin.Context) {
	userID := c.Param("user_id")
	role, err := h.repo.GetUserCustomRole(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Failed to fetch role", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "User custom role", role)
}

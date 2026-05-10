package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// BackupHandler exposes super_admin-only database backup management:
// list, trigger, and download. Restore is intentionally absent — that is
// a privileged operator action that must run from a trusted shell, not a
// dashboard button.
type BackupHandler struct {
	svc    *services.BackupService
	logger *zap.Logger
}

func NewBackupHandler(svc *services.BackupService, logger *zap.Logger) *BackupHandler {
	return &BackupHandler{svc: svc, logger: logger}
}

// List godoc
// @Router /admin/system/backups [get]
func (h *BackupHandler) List(c *gin.Context) {
	rows, err := h.svc.List(c.Request.Context(), 50)
	if err != nil {
		h.logger.Error("backup list failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", utils.ErrInternalServer)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"backups": rows})
}

// Run godoc
// @Router /admin/system/backups/run [post]
func (h *BackupHandler) Run(c *gin.Context) {
	adminIDValue, _ := c.Get("user_id")
	var adminID *string
	if v, ok := adminIDValue.(string); ok && v != "" {
		adminID = &v
	}

	id, err := h.svc.Run(c.Request.Context(), "admin", adminID)
	if err != nil {
		if errors.Is(err, services.ErrBackupDisabled) {
			utils.SendError(c, http.StatusFailedDependency,
				"Backups disabled — set BACKUP_ENABLED=true and BACKUP_PASSPHRASE",
				err,
			)
			return
		}
		h.logger.Error("backup run failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Backup failed", err)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Backup started", gin.H{"id": id.String()})
}

// Download godoc
// Streams the encrypted dump artifact straight to the admin's browser.
// Streaming (vs. a presigned URL) keeps the artifact behind admin auth
// and avoids the internal-hostname problem the embedded MinIO endpoint
// would create with signed URLs.
// @Router /admin/system/backups/{id}/download [get]
func (h *BackupHandler) Download(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.SendError(c, http.StatusBadRequest, "id required", utils.ErrValidation)
		return
	}
	stream, err := h.svc.OpenDownload(c.Request.Context(), id)
	if err != nil {
		utils.SendError(c, http.StatusNotFound, err.Error(), utils.ErrNotFound)
		return
	}
	defer stream.Reader.Close()

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", `attachment; filename="`+stream.Filename+`"`)
	c.Header("Cache-Control", "no-store")
	c.DataFromReader(http.StatusOK, stream.Size, "application/octet-stream", stream.Reader, nil)
}

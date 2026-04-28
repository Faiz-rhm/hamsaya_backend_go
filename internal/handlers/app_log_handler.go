package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
)

// AppLogHandler exposes /admin/logs — recent warn+ application log entries.
// Pairs with observability.DBLogSink which writes to the table out-of-band.
type AppLogHandler struct {
	repo   repositories.AppLogRepository
	logger *zap.Logger
}

func NewAppLogHandler(repo repositories.AppLogRepository, logger *zap.Logger) *AppLogHandler {
	return &AppLogHandler{repo: repo, logger: logger}
}

// List returns paginated log entries with optional level / request_id /
// free-text filters. Empty `level` returns all (warn+ in practice).
func (h *AppLogHandler) List(c *gin.Context) {
	level := strings.ToLower(strings.TrimSpace(c.Query("level")))
	switch level {
	case "", "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
	default:
		level = ""
	}

	page := atoiOr(c.Query("page"), 1)
	limit := atoiOr(c.Query("limit"), 50)

	filter := repositories.AppLogFilter{
		Level:     level,
		Search:    strings.TrimSpace(c.Query("search")),
		RequestID: strings.TrimSpace(c.Query("request_id")),
		Page:      page,
		Limit:     limit,
	}

	entries, total, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("list app logs", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to load logs", err)
		return
	}

	if entries == nil {
		entries = []*repositories.AppLogEntry{}
	}

	totalPages := 0
	if filter.Limit > 0 {
		totalPages = (total + filter.Limit - 1) / filter.Limit
	}

	utils.SendSuccess(c, http.StatusOK, "Logs retrieved", gin.H{
		"items":       entries,
		"total_count": total,
		"page":        filter.Page,
		"limit":       filter.Limit,
		"total_pages": totalPages,
	})
}

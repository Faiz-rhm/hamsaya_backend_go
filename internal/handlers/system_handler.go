package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SystemHandler exposes super_admin-only platform telemetry: build info,
// service health, table-row stats, feature flags, and global session
// management. All endpoints assume the caller has already passed
// AuthMiddleware.RequireSuperAdmin().
type SystemHandler struct {
	db          *database.DB
	redis       *redis.Client
	flagRepo    repositories.FeatureFlagRepository
	logger      *zap.Logger
	startedAt   time.Time
}

func NewSystemHandler(
	db *database.DB,
	redis *redis.Client,
	flagRepo repositories.FeatureFlagRepository,
	logger *zap.Logger,
) *SystemHandler {
	return &SystemHandler{
		db:        db,
		redis:     redis,
		flagRepo:  flagRepo,
		logger:    logger,
		startedAt: time.Now(),
	}
}

// BuildInfo returns ldflags-injected build metadata + runtime info, surfaced
// to the /system page so super_admins can confirm what is actually running.
// @Router /admin/system/build-info [get]
func (h *SystemHandler) BuildInfo(c *gin.Context) {
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{
		"version":     version,
		"build_time":  buildTime,
		"git_commit":  gitCommit,
		"go_version":  runtime.Version(),
		"go_arch":     runtime.GOARCH,
		"go_os":       runtime.GOOS,
		"started_at":  h.startedAt.UTC(),
		"uptime_secs": int64(time.Since(h.startedAt).Seconds()),
	})
}

// ServiceHealth aggregates DB pool stats, Redis ping latency, and goroutine
// count. Distinct from /health/* endpoints which target probes — this is the
// human-readable system overview.
// @Router /admin/system/health [get]
func (h *SystemHandler) ServiceHealth(c *gin.Context) {
	ctx := c.Request.Context()

	dbStat := h.db.Pool.Stat()
	dbInfo := gin.H{
		"acquired_conns":      dbStat.AcquiredConns(),
		"idle_conns":          dbStat.IdleConns(),
		"max_conns":           dbStat.MaxConns(),
		"total_conns":         dbStat.TotalConns(),
		"acquire_count":       dbStat.AcquireCount(),
		"acquire_duration_ms": dbStat.AcquireDuration().Milliseconds(),
	}

	redisInfo := gin.H{"available": false}
	if h.redis != nil {
		start := time.Now()
		if err := h.redis.Ping(ctx).Err(); err == nil {
			redisInfo = gin.H{
				"available":  true,
				"latency_ms": time.Since(start).Milliseconds(),
			}
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{
		"db":               dbInfo,
		"redis":            redisInfo,
		"goroutines":       runtime.NumGoroutine(),
		"heap_alloc_bytes": mem.HeapAlloc,
		"heap_sys_bytes":   mem.HeapSys,
		"gc_pause_ns_p99":  mem.PauseNs[(mem.NumGC+255)%256],
	})
}

// TableStats returns row counts for every public-schema table. Useful
// signal for quick sanity checks on prod scale.
// @Router /admin/system/table-stats [get]
func (h *SystemHandler) TableStats(c *gin.Context) {
	rows, err := h.db.Pool.Query(c.Request.Context(), `
		SELECT relname AS table, n_live_tup AS rows, last_autovacuum, last_autoanalyze
		FROM pg_stat_user_tables
		ORDER BY n_live_tup DESC
	`)
	if err != nil {
		h.logger.Error("table-stats query failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", utils.ErrInternalServer)
		return
	}
	defer rows.Close()

	type entry struct {
		Table          string     `json:"table"`
		Rows           int64      `json:"rows"`
		LastAutovacuum *time.Time `json:"last_autovacuum,omitempty"`
		LastAutoanalyze *time.Time `json:"last_autoanalyze,omitempty"`
	}

	out := make([]entry, 0, 64)
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.Table, &e.Rows, &e.LastAutovacuum, &e.LastAutoanalyze); err != nil {
			h.logger.Error("table-stats scan failed", zap.Error(err))
			utils.SendError(c, http.StatusInternalServerError, "Scan failed", utils.ErrInternalServer)
			return
		}
		out = append(out, e)
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"tables": out})
}

// SessionsList returns every non-revoked, non-expired session across all
// users — for super_admin "force log everyone out" or anomaly review.
// @Router /admin/system/sessions [get]
func (h *SystemHandler) SessionsList(c *gin.Context) {
	rows, err := h.db.Pool.Query(c.Request.Context(), `
		SELECT s.id::text, s.user_id::text, COALESCE(u.email,''),
		       s.device_info, s.ip_address, s.user_agent,
		       s.created_at, s.expires_at
		FROM user_sessions s
		LEFT JOIN users u ON u.id = s.user_id
		WHERE s.revoked = FALSE AND s.expires_at > NOW()
		ORDER BY s.created_at DESC
		LIMIT 500
	`)
	if err != nil {
		h.logger.Error("sessions list query failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", utils.ErrInternalServer)
		return
	}
	defer rows.Close()

	type sess struct {
		ID         string    `json:"id"`
		UserID     string    `json:"user_id"`
		Email      string    `json:"email"`
		DeviceInfo *string   `json:"device_info,omitempty"`
		IPAddress  *string   `json:"ip_address,omitempty"`
		UserAgent  *string   `json:"user_agent,omitempty"`
		CreatedAt  time.Time `json:"created_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	}
	out := make([]sess, 0, 100)
	for rows.Next() {
		var s sess
		if err := rows.Scan(&s.ID, &s.UserID, &s.Email, &s.DeviceInfo, &s.IPAddress, &s.UserAgent, &s.CreatedAt, &s.ExpiresAt); err != nil {
			h.logger.Error("sessions scan failed", zap.Error(err))
			utils.SendError(c, http.StatusInternalServerError, "Scan failed", utils.ErrInternalServer)
			return
		}
		out = append(out, s)
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"sessions": out, "count": len(out)})
}

// SessionRevoke marks a single session revoked. Lighter-weight than the
// per-user /auth/logout-all path since it operates on session_id directly.
// @Router /admin/system/sessions/{session_id}/revoke [post]
func (h *SystemHandler) SessionRevoke(c *gin.Context) {
	id := c.Param("session_id")
	if id == "" {
		utils.SendError(c, http.StatusBadRequest, "session_id required", utils.ErrValidation)
		return
	}
	tag, err := h.db.Pool.Exec(c.Request.Context(), `
		UPDATE user_sessions SET revoked = TRUE, revoked_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		h.logger.Error("session revoke failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Revoke failed", utils.ErrInternalServer)
		return
	}
	if tag.RowsAffected() == 0 {
		utils.SendError(c, http.StatusNotFound, "Session not found", utils.ErrNotFound)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Session revoked", nil)
}

// FlagsList returns every feature flag with its current value.
// @Router /admin/system/flags [get]
func (h *SystemHandler) FlagsList(c *gin.Context) {
	flags, err := h.flagRepo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("feature flags list failed", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Query failed", utils.ErrInternalServer)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"flags": flags})
}

// FlagsToggle flips a single feature flag. Body: {"enabled": bool}. Refuses
// unknown keys — flags must be defined via migrations so the catalog lives
// in source.
// @Router /admin/system/flags/{key} [put]
func (h *SystemHandler) FlagsToggle(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		utils.SendError(c, http.StatusBadRequest, "key required", utils.ErrValidation)
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid body", utils.ErrInvalidJSON)
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(string)
	if err := h.flagRepo.Set(c.Request.Context(), key, body.Enabled, uid); err != nil {
		h.logger.Warn("feature flag toggle rejected", zap.String("key", key), zap.Error(err))
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "Flag updated", nil)
}

// DenylistStats reports the size of the JWT access-token denylist (Redis).
// Useful for spotting runaway logout activity or a leaked token campaign.
// @Router /admin/system/denylist-stats [get]
func (h *SystemHandler) DenylistStats(c *gin.Context) {
	if h.redis == nil {
		utils.SendSuccess(c, http.StatusOK, "ok", gin.H{"available": false})
		return
	}
	keys, err := h.redis.Keys(c.Request.Context(), "denylist:*").Result()
	if err != nil {
		utils.SendError(c, http.StatusInternalServerError, "Redis query failed", utils.ErrInternalServer)
		return
	}
	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{
		"available": true,
		"count":     len(keys),
	})
}

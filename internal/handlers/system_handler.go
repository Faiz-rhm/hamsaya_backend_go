package handlers

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/hamsaya/backend/pkg/storage"
	"github.com/hamsaya/backend/pkg/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SystemHandler exposes super_admin-only platform telemetry: build info,
// service health, table-row stats, feature flags, and global session
// management. All endpoints assume the caller has already passed
// AuthMiddleware.RequireSuperAdmin().
type SystemHandler struct {
	db        *database.DB
	redis     *redis.Client
	flagRepo  repositories.FeatureFlagRepository
	hub       *websocket.Hub
	storage   *storage.Client
	logger    *zap.Logger
	startedAt time.Time
}

func NewSystemHandler(
	db *database.DB,
	redis *redis.Client,
	flagRepo repositories.FeatureFlagRepository,
	hub *websocket.Hub,
	storageClient *storage.Client,
	logger *zap.Logger,
) *SystemHandler {
	return &SystemHandler{
		db:        db,
		redis:     redis,
		flagRepo:  flagRepo,
		hub:       hub,
		storage:   storageClient,
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

// ServiceHealth aggregates DB pool stats, Redis ping latency + parsed INFO,
// goroutine + memory stats, WebSocket connection count, MinIO reachability,
// and a 1h app_logs error rate. Distinct from /health/* endpoints which
// target probes — this is the human-readable super_admin overview.
// @Router /admin/system/health [get]
func (h *SystemHandler) ServiceHealth(c *gin.Context) {
	ctx := c.Request.Context()

	dbStat := h.db.Pool.Stat()
	dbInfo := gin.H{
		"acquired_conns":              dbStat.AcquiredConns(),
		"idle_conns":                  dbStat.IdleConns(),
		"max_conns":                   dbStat.MaxConns(),
		"total_conns":                 dbStat.TotalConns(),
		"acquire_count":               dbStat.AcquireCount(),
		"acquire_duration_ms":         dbStat.AcquireDuration().Milliseconds(),
		"new_conns_count":             dbStat.NewConnsCount(),
		"empty_acquire_count":         dbStat.EmptyAcquireCount(),
		"canceled_acquire_count":      dbStat.CanceledAcquireCount(),
		"max_lifetime_destroy_count":  dbStat.MaxLifetimeDestroyCount(),
		"max_idle_destroy_count":      dbStat.MaxIdleDestroyCount(),
		"constructing_conns":          dbStat.ConstructingConns(),
	}

	redisInfo := gin.H{"available": false}
	if h.redis != nil {
		start := time.Now()
		if err := h.redis.Ping(ctx).Err(); err == nil {
			info := gin.H{
				"available":  true,
				"latency_ms": time.Since(start).Milliseconds(),
			}
			// Best-effort INFO parse — never fail health on a Redis quirk.
			if raw, err := h.redis.Info(ctx).Result(); err == nil {
				parsed := parseRedisInfo(raw)
				if v, ok := parsed["redis_version"]; ok {
					info["version"] = v
				}
				if v, ok := parsed["redis_mode"]; ok {
					info["mode"] = v
				}
				if v, ok := parsed["uptime_in_seconds"]; ok {
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						info["uptime_secs"] = n
					}
				}
				if v, ok := parsed["connected_clients"]; ok {
					if n, err := strconv.Atoi(v); err == nil {
						info["connected_clients"] = n
					}
				}
				if v, ok := parsed["maxclients"]; ok {
					if n, err := strconv.Atoi(v); err == nil {
						info["max_clients"] = n
					}
				}
				if v, ok := parsed["used_memory"]; ok {
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						info["used_memory_bytes"] = n
					}
				}
				if v, ok := parsed["used_memory_peak"]; ok {
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						info["used_memory_peak_bytes"] = n
					}
				}
				if v, ok := parsed["total_commands_processed"]; ok {
					if n, err := strconv.ParseInt(v, 10, 64); err == nil {
						info["total_commands_processed"] = n
					}
				}
				if v, ok := parsed["instantaneous_ops_per_sec"]; ok {
					if n, err := strconv.Atoi(v); err == nil {
						info["ops_per_sec"] = n
					}
				}
			}
			if size, err := h.redis.DBSize(ctx).Result(); err == nil {
				info["db_size"] = size
			}
			redisInfo = info
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	process := gin.H{
		"num_cpu":            runtime.NumCPU(),
		"num_gc":             mem.NumGC,
		"heap_objects":       mem.HeapObjects,
		"heap_idle_bytes":    mem.HeapIdle,
		"heap_inuse_bytes":   mem.HeapInuse,
		"heap_released_bytes": mem.HeapReleased,
		"total_alloc_bytes":  mem.TotalAlloc,
		"sys_bytes":          mem.Sys,
	}

	wsInfo := gin.H{"available": false}
	if h.hub != nil {
		wsInfo = gin.H{
			"available":   true,
			"connections": h.hub.ConnectionCount(),
			"shards":      h.hub.ShardCount(),
		}
	}

	storageInfo := gin.H{"configured": false}
	if h.storage != nil {
		st := h.storage.Stat(ctx)
		storageInfo = gin.H{
			"configured": true,
			"reachable":  st.Reachable,
			"endpoint":   st.Endpoint,
			"bucket":     st.Bucket,
			"use_ssl":    st.UseSSL,
			"latency_ms": st.LatencyMS,
		}
		if st.Error != "" {
			storageInfo["error"] = st.Error
		}
	}

	// 1h app_logs counts grouped by level. Failure here must not fail the
	// whole health response — fall through with zeros if the table query
	// errors (e.g., during migrations).
	logRate := gin.H{}
	rows, err := h.db.Pool.Query(ctx, `
		SELECT level, COUNT(*) FROM app_logs
		WHERE created_at > NOW() - INTERVAL '1 hour'
		GROUP BY level
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var level string
			var n int64
			if err := rows.Scan(&level, &n); err == nil {
				logRate[level] = n
			}
		}
	} else {
		h.logger.Warn("log-rate query failed", zap.Error(err))
	}

	utils.SendSuccess(c, http.StatusOK, "ok", gin.H{
		"db":               dbInfo,
		"redis":            redisInfo,
		"process":          process,
		"ws":               wsInfo,
		"storage":          storageInfo,
		"log_rate_1h":      logRate,
		"goroutines":       runtime.NumGoroutine(),
		"heap_alloc_bytes": mem.HeapAlloc,
		"heap_sys_bytes":   mem.HeapSys,
		"gc_pause_ns_p99":  mem.PauseNs[(mem.NumGC+255)%256],
	})
}

// parseRedisInfo walks a Redis INFO blob and returns a flat map of all
// `key:value` lines. Section headers (`# Server`) are ignored.
func parseRedisInfo(raw string) map[string]string {
	out := make(map[string]string, 64)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, ':'); i > 0 {
			out[line[:i]] = strings.TrimSpace(line[i+1:])
		}
	}
	return out
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
	// Cast JSONB device_info and inet ip_address to text so pgx can scan them
	// into *string without a custom codec.
	rows, err := h.db.Pool.Query(c.Request.Context(), `
		SELECT s.id::text, s.user_id::text, COALESCE(u.email,''),
		       s.device_info::text, s.ip_address::text, s.user_agent,
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

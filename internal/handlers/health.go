package handlers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/database"
	"github.com/redis/go-redis/v9"
)

var (
	// Build information (set via ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
	startTime = time.Now()
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db    *database.DB
	redis *redis.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *database.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

// Health handles the basic health check
// @Summary Health check
// @Description Check if the API is running
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	utils.SendSuccess(c, http.StatusOK, "OK", gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

// Live handles the liveness probe
// @Summary Liveness probe
// @Description Check if the application is alive
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health/live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	utils.SendSuccess(c, http.StatusOK, "OK", gin.H{
		"status": "alive",
	})
}

// Ready handles the readiness probe
// @Summary Readiness probe
// @Description Check if the application is ready to serve traffic
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} utils.Response
// @Router /health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	services := make(map[string]string)
	allHealthy := true

	// Check database
	if err := h.db.Health(ctx); err != nil {
		services["database"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		services["database"] = "healthy"
	}

	// Check Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		services["redis"] = "unhealthy: " + err.Error()
		allHealthy = false
	} else {
		services["redis"] = "healthy"
	}

	status := "ready"
	httpStatus := http.StatusOK
	message := "Service ready"

	if !allHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
		message = "Service degraded"
	}

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Services:  services,
	}

	c.JSON(httpStatus, gin.H{
		"success": allHealthy,
		"message": message,
		"data":    response,
	})
}

// DBStats returns database connection pool statistics
// @Summary Database statistics
// @Description Get database connection pool statistics
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health/db-stats [get]
func (h *HealthHandler) DBStats(c *gin.Context) {
	stats := h.db.Stats()

	utils.SendSuccess(c, http.StatusOK, "Database statistics", gin.H{
		"acquired_conns":             stats.AcquiredConns(),
		"canceled_acquire_count":     stats.CanceledAcquireCount(),
		"constructing_conns":         stats.ConstructingConns(),
		"empty_acquire_count":        stats.EmptyAcquireCount(),
		"idle_conns":                 stats.IdleConns(),
		"max_conns":                  stats.MaxConns(),
		"total_conns":                stats.TotalConns(),
		"new_conns_count":            stats.NewConnsCount(),
		"max_lifetime_destroy_count": stats.MaxLifetimeDestroyCount(),
		"max_idle_destroy_count":     stats.MaxIdleDestroyCount(),
	})
}

// RedisStats returns Redis server information and statistics
// @Summary Redis statistics
// @Description Get Redis server information and statistics
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health/redis-stats [get]
func (h *HealthHandler) RedisStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	// Get Redis INFO
	info, err := h.redis.Info(ctx).Result()
	if err != nil {
		utils.SendError(c, http.StatusServiceUnavailable, "Failed to get Redis stats", err)
		return
	}

	// Get Redis memory stats
	memStats, _ := h.redis.Info(ctx, "memory").Result()

	// Get key count from default DB
	dbSize, _ := h.redis.DBSize(ctx).Result()

	utils.SendSuccess(c, http.StatusOK, "Redis statistics", gin.H{
		"connected":   true,
		"db_size":     dbSize,
		"info":        info,
		"memory_info": memStats,
	})
}

// Version returns build and version information
// @Summary Version information
// @Description Get application version and build information
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health/version [get]
func (h *HealthHandler) Version(c *gin.Context) {
	utils.SendSuccess(c, http.StatusOK, "Version information", gin.H{
		"version":    version,
		"build_time": buildTime,
		"git_commit": gitCommit,
		"go_version": runtime.Version(),
	})
}

// Metrics returns system and runtime metrics
// @Summary System metrics
// @Description Get application runtime and system metrics
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health/metrics [get]
func (h *HealthHandler) Metrics(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(startTime)

	utils.SendSuccess(c, http.StatusOK, "System metrics", gin.H{
		"uptime_seconds": uptime.Seconds(),
		"uptime_human":   uptime.String(),
		"goroutines":     runtime.NumGoroutine(),
		"memory": gin.H{
			"alloc_mb":        m.Alloc / 1024 / 1024,
			"total_alloc_mb":  m.TotalAlloc / 1024 / 1024,
			"sys_mb":          m.Sys / 1024 / 1024,
			"num_gc":          m.NumGC,
			"gc_pause_ns":     m.PauseNs[(m.NumGC+255)%256],
			"heap_alloc_mb":   m.HeapAlloc / 1024 / 1024,
			"heap_sys_mb":     m.HeapSys / 1024 / 1024,
			"heap_idle_mb":    m.HeapIdle / 1024 / 1024,
			"heap_in_use_mb":  m.HeapInuse / 1024 / 1024,
			"heap_released_mb": m.HeapReleased / 1024 / 1024,
			"heap_objects":    m.HeapObjects,
		},
		"cpu": gin.H{
			"num_cpu": runtime.NumCPU(),
			"goos":    runtime.GOOS,
			"goarch":  runtime.GOARCH,
		},
	})
}

// Startup handles the startup probe
// @Summary Startup probe
// @Description Check if the application has started successfully
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} utils.Response
// @Router /health/startup [get]
func (h *HealthHandler) Startup(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Check critical services for startup
	if err := h.db.Health(ctx); err != nil {
		utils.SendError(c, http.StatusServiceUnavailable, "Database not ready", err)
		return
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		utils.SendError(c, http.StatusServiceUnavailable, "Redis not ready", err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Application started", gin.H{
		"status":     "started",
		"started_at": startTime,
		"uptime":     time.Since(startTime).String(),
	})
}

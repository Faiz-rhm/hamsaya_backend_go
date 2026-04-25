package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestRateLimiter(t *testing.T) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRateLimiter(client, zap.NewNop()), mr
}

func newRateLimitRouter(rl *RateLimiter, cfg RateLimitConfig) *gin.Engine {
	r := gin.New()
	r.Use(rl.Limit(cfg))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := RateLimitConfig{MaxRequests: 5, Window: time.Minute, KeyPrefix: "test:"}
	r := newRateLimitRouter(rl, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := RateLimitConfig{MaxRequests: 2, Window: time.Minute, KeyPrefix: "testblock:"}
	r := newRateLimitRouter(rl, cfg)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = fmt.Sprintf("10.0.0.2:%d", 1000+i)
		req.Header.Set("X-Forwarded-For", "10.0.0.2")
		r.ServeHTTP(w, req)
		if i < 2 {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.NotEmpty(t, w.Header().Get("Retry-After"))
		}
	}
}

func TestRateLimit_SetsHeaders(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := RateLimitConfig{MaxRequests: 10, Window: time.Minute, KeyPrefix: "testheaders:"}
	r := newRateLimitRouter(rl, cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.3")
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimit_LimitByTypeUnknownUsesDefault(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	r := gin.New()
	r.Use(rl.LimitByType("nonexistent"))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.4")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_LimitByUser_FallsBackToIPWhenNoUserID(t *testing.T) {
	rl, _ := newTestRateLimiter(t)
	cfg := RateLimitConfig{MaxRequests: 10, Window: time.Minute, KeyPrefix: "user:"}
	r := gin.New()
	r.Use(rl.LimitByUser(cfg))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.5")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

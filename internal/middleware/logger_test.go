package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newLoggerRouter(t *testing.T) *gin.Engine {
	logger := zap.NewNop().Sugar()
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/err", func(c *gin.Context) {
		_ = c.Error(assert.AnError)
		c.Status(http.StatusInternalServerError)
	})
	return r
}

func TestLogger_RequestProcessed(t *testing.T) {
	r := newLoggerRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogger_WithRequestID(t *testing.T) {
	logger := zap.NewNop().Sugar()
	r := gin.New()
	r.Use(RequestID())
	r.Use(Logger(logger))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "test-trace-id")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogger_WithErrors(t *testing.T) {
	r := newLoggerRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

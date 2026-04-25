package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/stretchr/testify/assert"
)

func newCORSRouter(cfg config.CORSConfig) *gin.Engine {
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.OPTIONS("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func defaultCORSConfig() config.CORSConfig {
	return config.CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}
}

func TestCORS_AllowedOrigin(t *testing.T) {
	r := newCORSRouter(defaultCORSConfig())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	r := newCORSRouter(defaultCORSConfig())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}
	r := newCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://anyone.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Preflight(t *testing.T) {
	r := newCORSRouter(defaultCORSConfig())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCORS_NoOriginHeader(t *testing.T) {
	r := newCORSRouter(defaultCORSConfig())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	// no Origin header — no ACAO header expected
	assert.Equal(t, http.StatusOK, w.Code)
}

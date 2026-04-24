package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestHealthHandler_Health(t *testing.T) {
	h := NewHealthHandler(nil, nil)
	r := gin.New()
	r.GET("/health", h.Health)

	w := doGet(r, "/health")

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "healthy", data["status"])
}

func TestHealthHandler_Live(t *testing.T) {
	h := NewHealthHandler(nil, nil)
	r := gin.New()
	r.GET("/health/live", h.Live)

	w := doGet(r, "/health/live")

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "alive", data["status"])
}

func TestHealthHandler_Version(t *testing.T) {
	h := NewHealthHandler(nil, nil)
	r := gin.New()
	r.GET("/health/version", h.Version)

	w := doGet(r, "/health/version")

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	data := body["data"].(map[string]interface{})
	assert.Contains(t, data, "version")
	assert.Contains(t, data, "go_version")
	assert.Contains(t, data, "build_time")
	assert.Contains(t, data, "git_commit")
}

func TestHealthHandler_Metrics(t *testing.T) {
	h := NewHealthHandler(nil, nil)
	r := gin.New()
	r.GET("/health/metrics", h.Metrics)

	w := doGet(r, "/health/metrics")

	assert.Equal(t, http.StatusOK, w.Code)
	body := parseBody(t, w)
	data := body["data"].(map[string]interface{})
	assert.Contains(t, data, "goroutines")
	assert.Contains(t, data, "memory")
	assert.Contains(t, data, "uptime_seconds")
	assert.Contains(t, data, "uptime_human")
	assert.Contains(t, data, "cpu")

	memory := data["memory"].(map[string]interface{})
	assert.Contains(t, memory, "alloc_mb")
	assert.Contains(t, memory, "heap_alloc_mb")
}

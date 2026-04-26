package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBodyLimitRouter(maxBytes int64) *gin.Engine {
	r := gin.New()
	r.Use(BodyLimit(maxBytes))
	r.POST("/echo", func(c *gin.Context) {
		// Force a read so MaxBytesReader actually trips on oversized bodies.
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", body)
	})
	return r
}

func TestBodyLimit_AllowsWithinLimit(t *testing.T) {
	r := newBodyLimitRouter(1024)
	w := httptest.NewRecorder()
	body := strings.Repeat("a", 512)
	req, _ := http.NewRequest(http.MethodPost, "/echo", bytes.NewBufferString(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, w.Body.String())
}

func TestBodyLimit_RejectsOversized(t *testing.T) {
	r := newBodyLimitRouter(64)
	w := httptest.NewRecorder()
	body := strings.Repeat("a", 1024)
	req, _ := http.NewRequest(http.MethodPost, "/echo", bytes.NewBufferString(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimit_NilBodyIsSafe(t *testing.T) {
	r := gin.New()
	r.Use(BodyLimit(1024))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/ok", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodyLimit_DefaultConstant(t *testing.T) {
	// 5 MB documented in body_limit.go — pin the contract so future bumps are deliberate.
	assert.Equal(t, int64(5<<20), int64(DefaultMaxBodyBytes))
}

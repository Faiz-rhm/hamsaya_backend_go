package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newRequestIDRouter() *gin.Engine {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("request_id"))
	})
	return r
}

func TestRequestID_GeneratesID(t *testing.T) {
	r := newRequestIDRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	id := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, id)
	// UUID format: 8-4-4-4-12
	assert.Len(t, id, 36)
	assert.Equal(t, id, w.Body.String()) // context value matches header
}

func TestRequestID_ReusesExistingID(t *testing.T) {
	r := newRequestIDRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	r.ServeHTTP(w, req)

	assert.Equal(t, "my-custom-id", w.Header().Get("X-Request-ID"))
	assert.Equal(t, "my-custom-id", w.Body.String())
}

func TestRequestID_DifferentPerRequest(t *testing.T) {
	r := newRequestIDRouter()

	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/test", nil))
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/test", nil))

	assert.NotEqual(t, w1.Header().Get("X-Request-ID"), w2.Header().Get("X-Request-ID"))
}

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTimeout_AttachesDeadline(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(50 * time.Millisecond))
	var deadlineSet bool
	r.GET("/x", func(c *gin.Context) {
		_, ok := c.Request.Context().Deadline()
		deadlineSet = ok
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, deadlineSet, "request context must carry a deadline")
}

func TestTimeout_TripsOnSlowHandler(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(20 * time.Millisecond))
	var ctxErr error
	r.GET("/slow", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			ctxErr = c.Request.Context().Err()
		case <-time.After(200 * time.Millisecond):
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/slow", nil)
	r.ServeHTTP(w, req)

	assert.ErrorIs(t, ctxErr, context.DeadlineExceeded,
		"handler must observe ctx.Done() before its own work finishes")
}

func TestTimeout_SkipsWebSocketUpgrade(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(10 * time.Millisecond))
	var deadlineSet bool
	r.GET("/ws", func(c *gin.Context) {
		_, ok := c.Request.Context().Deadline()
		deadlineSet = ok
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	r.ServeHTTP(w, req)

	assert.False(t, deadlineSet, "WebSocket upgrade must not inherit a request deadline")
}

func TestTimeout_DefaultConstant(t *testing.T) {
	// 25s documented in timeout.go — pin the contract.
	assert.Equal(t, 25*time.Second, DefaultRequestTimeout)
}

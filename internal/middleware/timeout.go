package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultRequestTimeout caps per-request work at the handler/service/repository
// chain. 25 seconds sits below the Gin server's 15s WriteTimeout for normal
// endpoints — wait, that's incompatible; see comment below.
//
// We pick 25s deliberately *higher* than ReadTimeout/WriteTimeout (15s) only
// because slow uploads to multipart endpoints need leniency at the transport
// layer. For inbound JSON requests the http.Server's WriteTimeout already
// short-circuits at 15s. The timeout here protects the database pool: a hung
// query inheriting this context is canceled instead of holding a connection
// forever.
const DefaultRequestTimeout = 25 * time.Second

// Timeout attaches a deadline to the request context so downstream services
// and repositories inherit a hard cap on how long a single request can run.
// Any pgx / redis call honoring context cancellation will return early when
// the deadline trips, freeing the connection for other requests.
//
// Long-lived endpoints (WebSocket upgrade, server-sent events, file streams)
// must opt out by registering a handler that does not pass through this
// middleware, or by replacing the request context before calling Next.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip on WebSocket upgrade — connections are long-lived by design.
		if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

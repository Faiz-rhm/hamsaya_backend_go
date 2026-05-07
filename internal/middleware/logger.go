package middleware

import (
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// sensitiveQueryKeys are query parameters whose values must never appear in
// access logs. The WS endpoint accepts `?token=<JWT>` for browser clients
// (Android also uses it as a fallback), and password-reset/email-verify links
// surface short-lived `code`/`token` values.
var sensitiveQueryKeys = map[string]struct{}{
	"token":  {},
	"code":   {},
	"email":  {},
	"secret": {},
}

// redactQuery returns the request's RawQuery with sensitive values replaced
// by "REDACTED". On parse error, returns "REDACTED" entirely (fail-closed).
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "REDACTED"
	}
	for k := range values {
		if _, ok := sensitiveQueryKeys[k]; ok {
			values.Set(k, "REDACTED")
		}
	}
	return values.Encode()
}

// Logger returns a gin middleware that logs HTTP requests with trace correlation
func Logger(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactQuery(c.Request.URL.RawQuery)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Build log fields
		fields := []interface{}{
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"request_id", c.GetString("request_id"),
		}

		// Add trace correlation if available (from OpenTelemetry)
		span := trace.SpanFromContext(c.Request.Context())
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			fields = append(fields,
				"trace_id", spanCtx.TraceID().String(),
				"span_id", spanCtx.SpanID().String(),
			)
		}

		// Add errors if any
		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
		}

		// Log request
		logger.Infow("HTTP Request", fields...)
	}
}

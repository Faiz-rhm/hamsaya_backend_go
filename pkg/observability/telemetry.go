package observability

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TelemetryProvider is the interface for observability operations (logging, tracing, metrics).
// Implementations can be the real Telemetry or NoopTelemetry for tests or when OTel is disabled.
type TelemetryProvider interface {
	LogInfo(msg string, keysAndValues ...interface{})
	LogError(err error, msg string, keysAndValues ...interface{})
	TraceStart(ctx context.Context, spanName string) (context.Context, trace.Span)
	Shutdown(ctx context.Context) error
	// Middleware for HTTP: request logging, duration histogram, in-flight counter
	LogRequest() gin.HandlerFunc
	MeterRequestDuration() gin.HandlerFunc
	MeterRequestsInFlight() gin.HandlerFunc
	// Metrics returns the application metrics (may be nil for NoopTelemetry when metrics disabled)
	Metrics() *Metrics
	// Stack returns the underlying OTel stack (may be nil for NoopTelemetry)
	Stack() *Stack
}

// Telemetry wraps the OTel stack with a unified API and optional Metrics for HTTP middleware.
type Telemetry struct {
	stack   *Stack
	log     *zap.SugaredLogger
	tracer  trace.Tracer
	cfg     Config
	metrics *Metrics
}

// NewTelemetry initializes the full observability stack (traces, metrics, logs) and returns a Telemetry
// that implements TelemetryProvider. If Init returns nil (observability disabled), use NewNoopTelemetry instead.
func NewTelemetry(ctx context.Context, cfg Config, logger *zap.Logger) (*Telemetry, error) {
	stack, err := Init(ctx, cfg, logger)
	if err != nil || stack == nil {
		return nil, err
	}

	sugared := logger.Sugar()
	// Use global providers (set by Init) for tracer and meter
	tracer := otel.Tracer(cfg.ServiceName, trace.WithInstrumentationVersion(cfg.ServiceVersion))

	metrics, err := NewMetrics(cfg.ServiceName)
	if err != nil {
		// Metrics registration failed; continue without custom HTTP metrics
		metrics = nil
	}

	return &Telemetry{
		stack:   stack,
		log:     sugared,
		tracer:  tracer,
		cfg:     cfg,
		metrics: metrics,
	}, nil
}

// LogInfo logs an info-level message with optional key-value pairs.
func (t *Telemetry) LogInfo(msg string, keysAndValues ...interface{}) {
	if t.log != nil {
		t.log.Infow(msg, keysAndValues...)
	}
}

// LogError logs an error with message and optional key-value pairs.
func (t *Telemetry) LogError(err error, msg string, keysAndValues ...interface{}) {
	if t.log != nil {
		t.log.Errorw(msg, append([]interface{}{"error", err}, keysAndValues...)...)
	}
}

// TraceStart starts a new span and returns the context with the span and the span.
// Callers must call span.End() when done (e.g. defer span.End()).
func (t *Telemetry) TraceStart(ctx context.Context, spanName string) (context.Context, trace.Span) {
	if t.tracer != nil {
		return t.tracer.Start(ctx, spanName)
	}
	return ctx, trace.SpanFromContext(ctx)
}

// Shutdown gracefully shuts down the observability stack.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.stack != nil {
		return t.stack.Shutdown(ctx)
	}
	return nil
}

// Metrics returns the application metrics (for custom recording). May be nil.
func (t *Telemetry) Metrics() *Metrics {
	return t.metrics
}

// Stack returns the underlying OTel stack for advanced use (e.g. GetPrometheusExporter).
func (t *Telemetry) Stack() *Stack {
	return t.stack
}

// LogRequest returns Gin middleware that logs each request with method, path, status, latency, and trace IDs.
func (t *Telemetry) LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		method := c.Request.Method
		reqID := c.GetString("request_id")

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		fields := []interface{}{
			"method", method,
			"path", path,
			"query", raw,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_ip", clientIP,
			"request_id", reqID,
		}
		if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().IsValid() {
			fields = append(fields, "trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String())
		}
		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
		}
		t.log.Infow("HTTP request", fields...)
	}
}

// MeterRequestDuration returns Gin middleware that records HTTP request duration and count via Metrics.
// No-op if Metrics is nil.
func (t *Telemetry) MeterRequestDuration() gin.HandlerFunc {
	m := t.metrics
	if m == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)
		var reqSize, respSize int64
		if c.Request.ContentLength > 0 {
			reqSize = c.Request.ContentLength
		}
		respSize = int64(c.Writer.Size())
		m.RecordHTTPRequest(c.Request.Context(), method, path, status, duration, reqSize, respSize)
	}
}

// MeterRequestsInFlight returns Gin middleware that increments/decrements the active request counter.
// No-op if Metrics is nil.
func (t *Telemetry) MeterRequestsInFlight() gin.HandlerFunc {
	m := t.metrics
	if m == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		attrs := []attribute.KeyValue{
			attribute.String("method", c.Request.Method),
			attribute.String("path", c.Request.URL.Path),
		}
		m.AddActiveRequest(ctx, attrs...)
		defer m.DoneActiveRequest(ctx, attrs...)
		c.Next()
	}
}

// NoopTelemetry implements TelemetryProvider with no-op behavior for tests or when OTel is disabled.
type NoopTelemetry struct {
	cfg Config
	log *zap.SugaredLogger
}

// NewNoopTelemetry returns a TelemetryProvider that does nothing except optional logging.
func NewNoopTelemetry(cfg Config, logger *zap.Logger) *NoopTelemetry {
	var sugared *zap.SugaredLogger
	if logger != nil {
		sugared = logger.Sugar()
	}
	return &NoopTelemetry{cfg: cfg, log: sugared}
}

// LogInfo is a no-op (or logs to zap if logger provided).
func (n *NoopTelemetry) LogInfo(msg string, keysAndValues ...interface{}) {
	if n.log != nil {
		n.log.Infow(msg, keysAndValues...)
	}
}

// LogError is a no-op (or logs to zap if logger provided).
func (n *NoopTelemetry) LogError(err error, msg string, keysAndValues ...interface{}) {
	if n.log != nil {
		n.log.Errorw(msg, append([]interface{}{"error", err}, keysAndValues...)...)
	}
}

// TraceStart returns the context and a no-op span.
func (n *NoopTelemetry) TraceStart(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

// Shutdown is a no-op.
func (n *NoopTelemetry) Shutdown(ctx context.Context) error {
	return nil
}

// LogRequest returns Gin middleware that only calls c.Next() (no logging unless you add it).
func (n *NoopTelemetry) LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// MeterRequestDuration returns Gin middleware that only calls c.Next().
func (n *NoopTelemetry) MeterRequestDuration() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// MeterRequestsInFlight returns Gin middleware that only calls c.Next().
func (n *NoopTelemetry) MeterRequestsInFlight() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// Metrics returns nil (no custom metrics).
func (n *NoopTelemetry) Metrics() *Metrics {
	return nil
}

// Stack returns nil.
func (n *NoopTelemetry) Stack() *Stack {
	return nil
}

// Ensure both types implement TelemetryProvider
var _ TelemetryProvider = (*Telemetry)(nil)
var _ TelemetryProvider = (*NoopTelemetry)(nil)

// RecordSpanError records an error on the span and sets status to Error.
func RecordSpanError(span trace.Span, err error) {
	if span == nil || !span.IsRecording() || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SpanAttributesFromGin returns common attributes for HTTP spans from Gin context.
func SpanAttributesFromGin(c *gin.Context) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", c.Request.Method),
		attribute.String("http.url_path", c.Request.URL.Path),
		attribute.Int("http.status_code", c.Writer.Status()),
	}
	if c.Request.URL.RawQuery != "" {
		attrs = append(attrs, attribute.String("http.query", c.Request.URL.RawQuery))
	}
	if rid := c.GetString("request_id"); rid != "" {
		attrs = append(attrs, attribute.String("request_id", rid))
	}
	return attrs
}

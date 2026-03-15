package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds commonly used application metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestSize     metric.Int64Histogram
	HTTPResponseSize    metric.Int64Histogram

	// Database metrics
	DBQueryDuration metric.Float64Histogram
	DBQueryTotal    metric.Int64Counter

	// Business metrics
	UsersCreated     metric.Int64Counter
	PostsCreated     metric.Int64Counter
	MessagesCreated  metric.Int64Counter
	ActiveWebSockets metric.Int64UpDownCounter
}

// NewMetrics creates and registers application metrics
func NewMetrics(serviceName string) (*Metrics, error) {
	meter := otel.Meter(serviceName)

	m := &Metrics{}
	var err error

	// HTTP metrics
	m.HTTPRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPRequestSize, err = meter.Int64Histogram(
		"http_request_size_bytes",
		metric.WithDescription("HTTP request size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	m.HTTPResponseSize, err = meter.Int64Histogram(
		"http_response_size_bytes",
		metric.WithDescription("HTTP response size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	// Database metrics
	m.DBQueryDuration, err = meter.Float64Histogram(
		"db_query_duration_seconds",
		metric.WithDescription("Database query duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1),
	)
	if err != nil {
		return nil, err
	}

	m.DBQueryTotal, err = meter.Int64Counter(
		"db_queries_total",
		metric.WithDescription("Total number of database queries"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return nil, err
	}

	// Business metrics
	m.UsersCreated, err = meter.Int64Counter(
		"users_created_total",
		metric.WithDescription("Total number of users created"),
		metric.WithUnit("{user}"),
	)
	if err != nil {
		return nil, err
	}

	m.PostsCreated, err = meter.Int64Counter(
		"posts_created_total",
		metric.WithDescription("Total number of posts created"),
		metric.WithUnit("{post}"),
	)
	if err != nil {
		return nil, err
	}

	m.MessagesCreated, err = meter.Int64Counter(
		"messages_created_total",
		metric.WithDescription("Total number of chat messages created"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return nil, err
	}

	m.ActiveWebSockets, err = meter.Int64UpDownCounter(
		"websocket_connections_active",
		metric.WithDescription("Number of active WebSocket connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordHTTPRequest records metrics for an HTTP request
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
	}

	m.HTTPRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.HTTPRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if requestSize > 0 {
		m.HTTPRequestSize.Record(ctx, requestSize, metric.WithAttributes(attrs...))
	}
	if responseSize > 0 {
		m.HTTPResponseSize.Record(ctx, responseSize, metric.WithAttributes(attrs...))
	}
}

// RecordDBQuery records metrics for a database query
func (m *Metrics) RecordDBQuery(ctx context.Context, operation, table string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("table", table),
		attribute.Bool("success", success),
	}

	m.DBQueryTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.DBQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordUserCreated increments the user creation counter
func (m *Metrics) RecordUserCreated(ctx context.Context, provider string) {
	m.UsersCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("provider", provider),
	))
}

// RecordPostCreated increments the post creation counter
func (m *Metrics) RecordPostCreated(ctx context.Context, postType string) {
	m.PostsCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", postType),
	))
}

// RecordMessageCreated increments the message creation counter
func (m *Metrics) RecordMessageCreated(ctx context.Context) {
	m.MessagesCreated.Add(ctx, 1)
}

// WebSocketConnected increments the active WebSocket connections counter
func (m *Metrics) WebSocketConnected(ctx context.Context) {
	m.ActiveWebSockets.Add(ctx, 1)
}

// WebSocketDisconnected decrements the active WebSocket connections counter
func (m *Metrics) WebSocketDisconnected(ctx context.Context) {
	m.ActiveWebSockets.Add(ctx, -1)
}

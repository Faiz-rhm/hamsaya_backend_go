package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/zap"
)

// Config holds configuration for the observability stack
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SamplingRate   float64
	Enabled        bool
}

// Stack holds the initialized observability providers
type Stack struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	PromExporter   *prometheus.Exporter
	logger         *zap.Logger
}

// Init initializes the OpenTelemetry observability stack with tracing and metrics
func Init(ctx context.Context, cfg Config, logger *zap.Logger) (*Stack, error) {
	if !cfg.Enabled {
		logger.Info("Observability disabled")
		return nil, nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("service.instance.id", generateInstanceID()),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	stack := &Stack{logger: logger}

	// Initialize tracing if OTLP endpoint is configured
	if cfg.OTLPEndpoint != "" {
		tp, err := initTracing(ctx, cfg, res)
		if err != nil {
			logger.Warn("Failed to initialize tracing, continuing without it", zap.Error(err))
		} else {
			stack.TracerProvider = tp
			otel.SetTracerProvider(tp)
			logger.Info("OpenTelemetry tracing initialized",
				zap.String("endpoint", cfg.OTLPEndpoint),
				zap.Float64("sampling_rate", cfg.SamplingRate),
			)
		}
	}

	// Set up propagation (always, even if tracing export fails)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize Prometheus metrics
	mp, promExporter, err := initMetrics(ctx, res)
	if err != nil {
		logger.Warn("Failed to initialize metrics, continuing without them", zap.Error(err))
	} else {
		stack.MeterProvider = mp
		stack.PromExporter = promExporter
		otel.SetMeterProvider(mp)
		logger.Info("OpenTelemetry metrics initialized (Prometheus exporter)")
	}

	return stack, nil
}

// initTracing sets up the OpenTelemetry trace provider with OTLP exporter
func initTracing(ctx context.Context, cfg Config, res *resource.Resource) (*trace.TracerProvider, error) {
	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // Use WithTLSCredentials for production with TLS
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configure sampler based on sampling rate
	var sampler trace.Sampler
	if cfg.SamplingRate >= 1.0 {
		sampler = trace.AlwaysSample()
	} else if cfg.SamplingRate <= 0 {
		sampler = trace.NeverSample()
	} else {
		sampler = trace.TraceIDRatioBased(cfg.SamplingRate)
	}

	// Create trace provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(5*time.Second),
			trace.WithMaxExportBatchSize(512),
		),
		trace.WithResource(res),
		trace.WithSampler(trace.ParentBased(sampler)),
	)

	return tp, nil
}

// initMetrics sets up the OpenTelemetry meter provider with Prometheus exporter
func initMetrics(ctx context.Context, res *resource.Resource) (*metric.MeterProvider, *prometheus.Exporter, error) {
	// Create Prometheus exporter
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider
	mp := metric.NewMeterProvider(
		metric.WithReader(promExporter),
		metric.WithResource(res),
	)

	return mp, promExporter, nil
}

// Shutdown gracefully shuts down the observability stack
func (s *Stack) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	var errs []error

	if s.TracerProvider != nil {
		if err := s.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
	}

	if s.MeterProvider != nil {
		if err := s.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("observability shutdown errors: %v", errs)
	}

	if s.logger != nil {
		s.logger.Info("Observability stack shut down successfully")
	}

	return nil
}

// generateInstanceID creates a unique instance identifier
func generateInstanceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// GetPrometheusExporter returns the Prometheus exporter for use with HTTP handler
func (s *Stack) GetPrometheusExporter() *prometheus.Exporter {
	if s == nil {
		return nil
	}
	return s.PromExporter
}

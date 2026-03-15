package observability

import (
	"fmt"
	"os"
	"strconv"
)

// Env keys for observability configuration (optional; can still pass Config from app config).
const (
	EnvServiceName    = "SERVICE_NAME"
	EnvServiceVersion = "SERVICE_VERSION"
	EnvEnvironment    = "ENV"
	EnvOTLPEndpoint   = "OTLP_ENDPOINT"
	EnvTraceSampling  = "TRACE_SAMPLING_RATE"
	EnvEnabled        = "OBSERVABILITY_ENABLED"
)

// NewConfigFromEnv loads Config from environment variables.
// Use this when you want the observability package to be self-contained; otherwise
// build Config from your app config (e.g. Viper) and pass to NewTelemetry/Init.
func NewConfigFromEnv() (Config, error) {
	cfg := Config{
		ServiceName:    getEnv(EnvServiceName, "my-go-app"),
		ServiceVersion: getEnv(EnvServiceVersion, "0.1.0"),
		Environment:    getEnv(EnvEnvironment, "development"),
		OTLPEndpoint:   os.Getenv(EnvOTLPEndpoint),
		SamplingRate:   1.0,
		Enabled:        true,
	}

	if v := os.Getenv(EnvTraceSampling); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("%s: %w", EnvTraceSampling, err)
		}
		cfg.SamplingRate = f
	}

	if v := os.Getenv(EnvEnabled); v != "" {
		cfg.Enabled = v == "1" || v == "true" || v == "yes"
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

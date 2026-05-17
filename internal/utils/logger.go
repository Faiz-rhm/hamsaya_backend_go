package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger
var baseLogger *zap.Logger

// InitLogger initializes the global logger.
//
// In production (ENV=production) the logger emits JSON without ANSI color
// codes, runs in non-development mode (no DPanic stack traces), and defaults
// to InfoLevel when LOG_LEVEL is unset — keeping debug spam out of the
// shipped server logs and aligning with log-aggregator expectations.
// In any other environment the legacy human-friendly console encoder is
// used so local development still gets coloured, readable output.
func InitLogger(level string) error {
	isProduction := os.Getenv("ENV") == "production"

	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		if isProduction {
			zapLevel = zapcore.InfoLevel
		} else {
			zapLevel = zapcore.InfoLevel
		}
	}

	levelEncoder := zapcore.CapitalColorLevelEncoder
	encoding := "console"
	development := true
	if isProduction {
		levelEncoder = zapcore.CapitalLevelEncoder
		encoding = "json"
		development = false
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    levelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      development,
		Encoding:         encoding,
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		return err
	}

	baseLogger = logger
	Logger = logger.Sugar()
	return nil
}

// GetLogger returns the global sugared logger instance
func GetLogger() *zap.SugaredLogger {
	if Logger == nil {
		// Fallback to a basic logger if not initialized
		logger, _ := zap.NewProduction()
		baseLogger = logger
		Logger = logger.Sugar()
	}
	return Logger
}

// GetBaseLogger returns the global unsugared logger instance
func GetBaseLogger() *zap.Logger {
	if baseLogger == nil {
		// Fallback to a basic logger if not initialized
		logger, _ := zap.NewProduction()
		baseLogger = logger
		Logger = logger.Sugar()
	}
	return baseLogger
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
	if baseLogger != nil {
		_ = baseLogger.Sync()
	}
}

// SetLogLevel dynamically sets the log level
func SetLogLevel(level string) {
	_ = os.Setenv("LOG_LEVEL", level)
}

// WrapWithCore returns a new logger that tees its output through `extra`
// alongside the existing core. Used by main.go to attach the DB log sink
// once the database is ready, so warn+ entries are mirrored to app_logs.
func WrapWithCore(base *zap.Logger, extra zapcore.Core) *zap.Logger {
	if base == nil || extra == nil {
		return base
	}
	wrapped := base.WithOptions(zap.WrapCore(func(existing zapcore.Core) zapcore.Core {
		return zapcore.NewTee(existing, extra)
	}))
	baseLogger = wrapped
	Logger = wrapped.Sugar()
	return wrapped
}

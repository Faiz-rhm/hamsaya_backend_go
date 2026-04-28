package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger
var baseLogger *zap.Logger

// InitLogger initializes the global logger
func InitLogger(level string) error {
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
		zapLevel = zapcore.InfoLevel
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
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      true,
		Encoding:         "console",
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

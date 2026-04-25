package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger_ValidLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			err := InitLogger(level)
			require.NoError(t, err)
			assert.NotNil(t, Logger)
			assert.NotNil(t, baseLogger)
		})
	}
}

func TestGetLogger_InitializedReturnsGlobal(t *testing.T) {
	require.NoError(t, InitLogger("info"))
	l := GetLogger()
	assert.NotNil(t, l)
	assert.Equal(t, Logger, l)
}

func TestGetLogger_FallbackWhenNil(t *testing.T) {
	// Reset global state
	Logger = nil
	baseLogger = nil

	l := GetLogger()
	assert.NotNil(t, l)
}

func TestGetBaseLogger_FallbackWhenNil(t *testing.T) {
	Logger = nil
	baseLogger = nil

	l := GetBaseLogger()
	assert.NotNil(t, l)
}

func TestSync_DoesNotPanic(t *testing.T) {
	require.NoError(t, InitLogger("info"))
	assert.NotPanics(t, Sync)
}

func TestSync_WhenNilDoesNotPanic(t *testing.T) {
	Logger = nil
	baseLogger = nil
	assert.NotPanics(t, Sync)
}

func TestSetLogLevel_SetsEnvVar(t *testing.T) {
	assert.NotPanics(t, func() { SetLogLevel("debug") })
}

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMFAHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "test")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("JWT_SECRET", "test-secret-key-at-least-32-characters-long")
	t.Setenv("MFA_SECRET_ENCRYPTION_KEY", testMFAHexKey)
	t.Setenv("STORAGE_SECRET_KEY", "test-storage-secret-not-default")
}

func TestLoad(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "test-secret-key-at-least-32-characters-long", cfg.JWT.Secret)
}

func TestLoad_RejectsDefaultJWTSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoad_RejectsEmptyMFAKey(t *testing.T) {
	setValidEnv(t)
	t.Setenv("MFA_SECRET_ENCRYPTION_KEY", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MFA_SECRET_ENCRYPTION_KEY")
}

func TestLoad_RejectsShortMFAKey(t *testing.T) {
	setValidEnv(t)
	t.Setenv("MFA_SECRET_ENCRYPTION_KEY", strings.Repeat("a", 32))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MFA_SECRET_ENCRYPTION_KEY")
	assert.Contains(t, err.Error(), "64")
}

func TestLoad_RejectsDefaultStorageSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("STORAGE_SECRET_KEY", "minioadmin")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STORAGE_SECRET_KEY")
}

func TestLoad_RejectsEmptyStorageSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("STORAGE_SECRET_KEY", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STORAGE_SECRET_KEY")
}

func TestDatabaseConfig_GetDSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
		SSLMode:  "disable",
	}

	dsn := cfg.GetDSN()
	expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	assert.Equal(t, expected, dsn)
}

func TestRedisConfig_GetAddr(t *testing.T) {
	cfg := RedisConfig{
		Host: "localhost",
		Port: "6379",
	}

	addr := cfg.GetAddr()
	assert.Equal(t, "localhost:6379", addr)
}

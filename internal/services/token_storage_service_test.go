package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestRedis(t *testing.T) (*TokenStorageService, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewTokenStorageService(rdb, zap.NewNop()), mr
}

func TestTokenStorageService_VerificationToken(t *testing.T) {
	svc, mr := newTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("store and retrieve", func(t *testing.T) {
		err := svc.StoreVerificationToken(ctx, "user-1", "123456", time.Hour)
		require.NoError(t, err)

		userID, err := svc.GetUserIDFromVerificationToken(ctx, "123456")
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetUserIDFromVerificationToken(ctx, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or expired")
	})

	t.Run("delete", func(t *testing.T) {
		err := svc.StoreVerificationToken(ctx, "user-2", "654321", time.Hour)
		require.NoError(t, err)

		err = svc.DeleteVerificationToken(ctx, "654321")
		require.NoError(t, err)

		_, err = svc.GetUserIDFromVerificationToken(ctx, "654321")
		require.Error(t, err)
	})

	t.Run("expired", func(t *testing.T) {
		err := svc.StoreVerificationToken(ctx, "user-3", "999999", time.Millisecond)
		require.NoError(t, err)

		mr.FastForward(time.Second)

		_, err = svc.GetUserIDFromVerificationToken(ctx, "999999")
		require.Error(t, err)
	})
}

func TestTokenStorageService_PasswordResetToken(t *testing.T) {
	svc, mr := newTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("store and retrieve", func(t *testing.T) {
		err := svc.StorePasswordResetToken(ctx, "user-1", "reset123", time.Hour)
		require.NoError(t, err)

		userID, err := svc.GetUserIDFromPasswordResetToken(ctx, "reset123")
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetUserIDFromPasswordResetToken(ctx, "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or expired")
	})

	t.Run("delete", func(t *testing.T) {
		err := svc.StorePasswordResetToken(ctx, "user-2", "todelete", time.Hour)
		require.NoError(t, err)

		err = svc.DeletePasswordResetToken(ctx, "todelete")
		require.NoError(t, err)

		_, err = svc.GetUserIDFromPasswordResetToken(ctx, "todelete")
		require.Error(t, err)
	})
}

func TestTokenStorageService_MFAChallenge(t *testing.T) {
	svc, mr := newTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("store and retrieve", func(t *testing.T) {
		err := svc.StoreMFAChallenge(ctx, "challenge-1", "user-1", 5*time.Minute)
		require.NoError(t, err)

		userID, err := svc.GetUserIDFromMFAChallenge(ctx, "challenge-1")
		require.NoError(t, err)
		assert.Equal(t, "user-1", userID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetUserIDFromMFAChallenge(ctx, "no-such-challenge")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or expired")
	})

	t.Run("delete", func(t *testing.T) {
		err := svc.StoreMFAChallenge(ctx, "challenge-del", "user-2", time.Hour)
		require.NoError(t, err)

		err = svc.DeleteMFAChallenge(ctx, "challenge-del")
		require.NoError(t, err)

		_, err = svc.GetUserIDFromMFAChallenge(ctx, "challenge-del")
		require.Error(t, err)
	})
}

func TestTokenStorageService_Blacklist(t *testing.T) {
	svc, mr := newTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("not blacklisted initially", func(t *testing.T) {
		blacklisted, err := svc.IsTokenBlacklisted(ctx, "hash-abc")
		require.NoError(t, err)
		assert.False(t, blacklisted)
	})

	t.Run("blacklist and check", func(t *testing.T) {
		err := svc.BlacklistToken(ctx, "hash-xyz", time.Hour)
		require.NoError(t, err)

		blacklisted, err := svc.IsTokenBlacklisted(ctx, "hash-xyz")
		require.NoError(t, err)
		assert.True(t, blacklisted)
	})

	t.Run("expires", func(t *testing.T) {
		err := svc.BlacklistToken(ctx, "hash-exp", time.Millisecond)
		require.NoError(t, err)

		mr.FastForward(time.Second)

		blacklisted, err := svc.IsTokenBlacklisted(ctx, "hash-exp")
		require.NoError(t, err)
		assert.False(t, blacklisted)
	})
}

func TestTokenStorageService_SessionCache(t *testing.T) {
	svc, mr := newTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("cache miss returns nil", func(t *testing.T) {
		data, err := svc.GetCachedSession(ctx, "missing-session")
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("store and retrieve", func(t *testing.T) {
		expected := &SessionCacheData{
			UserID:          "user-1",
			AccessTokenHash: "hash-abc",
			Revoked:         false,
			ExpiresAt:       time.Unix(time.Now().Add(time.Hour).Unix(), 0),
		}
		err := svc.CacheSession(ctx, "session-1", expected)
		require.NoError(t, err)

		got, err := svc.GetCachedSession(ctx, "session-1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, expected.UserID, got.UserID)
		assert.Equal(t, expected.AccessTokenHash, got.AccessTokenHash)
		assert.Equal(t, expected.Revoked, got.Revoked)
		assert.Equal(t, expected.ExpiresAt.Unix(), got.ExpiresAt.Unix())
	})

	t.Run("revoked session cached", func(t *testing.T) {
		data := &SessionCacheData{
			UserID:          "user-2",
			AccessTokenHash: "hash-revoked",
			Revoked:         true,
			ExpiresAt:       time.Unix(time.Now().Add(time.Hour).Unix(), 0),
		}
		err := svc.CacheSession(ctx, "session-revoked", data)
		require.NoError(t, err)

		got, err := svc.GetCachedSession(ctx, "session-revoked")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.Revoked)
	})

	t.Run("invalidate", func(t *testing.T) {
		data := &SessionCacheData{
			UserID:          "user-3",
			AccessTokenHash: "hash-del",
			Revoked:         false,
			ExpiresAt:       time.Unix(time.Now().Add(time.Hour).Unix(), 0),
		}
		err := svc.CacheSession(ctx, "session-del", data)
		require.NoError(t, err)

		err = svc.InvalidateSessionCache(ctx, "session-del")
		require.NoError(t, err)

		got, err := svc.GetCachedSession(ctx, "session-del")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestTokenStorageService_Redis_Failure(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:0",
		MaxRetries:  0,
		DialTimeout: time.Millisecond,
	})
	svc := NewTokenStorageService(rdb, zap.NewNop())
	ctx := context.Background()

	t.Run("store verification token fails", func(t *testing.T) {
		err := svc.StoreVerificationToken(ctx, "user-1", "code", time.Hour)
		require.Error(t, err)
	})

	t.Run("get verification token fails", func(t *testing.T) {
		_, err := svc.GetUserIDFromVerificationToken(ctx, "code")
		require.Error(t, err)
	})

	t.Run("blacklist check fails", func(t *testing.T) {
		_, err := svc.IsTokenBlacklisted(ctx, "hash")
		require.Error(t, err)
	})
}

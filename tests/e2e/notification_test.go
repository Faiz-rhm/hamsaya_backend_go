package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_Notification_GetNotifications(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notif-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notif-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/notifications"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "get notifications failed: %s", string(raw))
}

func TestE2E_Notification_GetUnreadCount(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifcount-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifcount-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/notifications/unread-count"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "unread count failed: %s", string(raw))

	var out struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.GreaterOrEqual(t, out.Data.Count, 0)
}

func TestE2E_Notification_MarkAllAsRead(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifread-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifread-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/notifications/read-all"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "mark all read failed: %s", string(raw))
}

func TestE2E_Notification_GetSettings(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifsettings-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifsettings-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/notifications/settings"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	// Settings may return 200 with empty or initialized list
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get settings failed: %s", string(raw))
}

func TestE2E_Notification_Unauthenticated(t *testing.T) {
	env := setupE2E(t)
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/notifications"), nil)
	resp := env.do(req)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

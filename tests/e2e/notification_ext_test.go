package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedNotification inserts a notification for userID directly in the DB
// so we have a real notification ID to work with.
func seedNotification(t *testing.T, env *testEnv, userID string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id string
	err := env.db.Pool.QueryRow(ctx, `
		INSERT INTO notifications (id, user_id, type, is_read, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'LIKE', false, now(), now())
		RETURNING id`,
		userID,
	).Scan(&id)
	if err != nil {
		t.Skipf("seedNotification: cannot insert notification: %v", err)
	}
	return id
}

func TestE2E_Notification_MarkSingleAsRead(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifread1-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifread1-%") })

	tokens := register(t, env, email, "Password123!")
	notifID := seedNotification(t, env, tokens.UserID)

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/notifications/"+notifID+"/read"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "mark-as-read failed: %s", string(raw))
}

func TestE2E_Notification_DeleteNotification(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifdel-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifdel-%") })

	tokens := register(t, env, email, "Password123!")
	notifID := seedNotification(t, env, tokens.UserID)

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/notifications/"+notifID), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete notification failed: %s", string(raw))
}

func TestE2E_Notification_UpdateSettings(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifsettingsupd-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifsettingsupd-%") })

	tokens := register(t, env, email, "Password123!")

	body := `{"category":"POSTS","push_pref":false}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/notifications/settings"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update settings failed: %s", string(raw))
}

func TestE2E_Notification_UnreadCountDecreasesAfterMarkAllRead(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-notifcount2-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-notifcount2-%") })

	tokens := register(t, env, email, "Password123!")
	seedNotification(t, env, tokens.UserID)
	seedNotification(t, env, tokens.UserID)

	// Get count before
	countResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/notifications/unread-count"), tokens.AccessToken, ""))
	defer func() { _ = countResp.Body.Close() }()
	countRaw, _ := io.ReadAll(countResp.Body)
	var countOut struct {
		Data struct{ Count int `json:"count"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(countRaw, &countOut))
	assert.GreaterOrEqual(t, countOut.Data.Count, 2)

	// Mark all read
	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/notifications/read-all"), tokens.AccessToken, "")).Body.Close()

	// Count should be 0
	countResp2 := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/notifications/unread-count"), tokens.AccessToken, ""))
	defer func() { _ = countResp2.Body.Close() }()
	countRaw2, _ := io.ReadAll(countResp2.Body)
	var countOut2 struct {
		Data struct{ Count int `json:"count"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(countRaw2, &countOut2))
	assert.Equal(t, 0, countOut2.Data.Count)
}

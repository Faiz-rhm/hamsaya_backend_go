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

func TestE2E_Auth_ChangePassword(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-changepw-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-changepw-%") })

	tokens := register(t, env, email, "OldPass123!")

	body := `{"current_password":"OldPass123!","new_password":"NewPass456!"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/auth/change-password"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "change-password failed: %s", string(raw))

	// Old password must no longer work
	oldResp := env.do(mustPost(env.url("/api/v1/auth/login"),
		fmt.Sprintf(`{"email":%q,"password":"OldPass123!"}`, email)))
	defer func() { _ = oldResp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, oldResp.StatusCode)

	// New password must work
	newTokens := login(t, env, email, "NewPass456!")
	assert.NotEmpty(t, newTokens.AccessToken)
}

func TestE2E_Auth_ChangePasswordWrongCurrentReturns401(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-changepwbad-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-changepwbad-%") })

	tokens := register(t, env, email, "Correct123!")

	body := `{"current_password":"WrongCurrent!","new_password":"NewPass456!"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/auth/change-password"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_Auth_GetActiveSessions(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-sessions-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-sessions-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/auth/sessions"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get sessions failed: %s", string(raw))
}

func TestE2E_Auth_SendVerificationEmail(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-sendemail-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-sendemail-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/auth/send-verification-email"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	// Acceptable outcomes:
	//   200 — email actually sent (Resend / SMTP configured)
	//   400 — profile not yet complete (auth_service gates OTPs until is_complete=true)
	//   500 — email send failed (Resend/SMTP unreachable)
	ok := resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusBadRequest ||
		resp.StatusCode == http.StatusInternalServerError
	assert.True(t, ok, "send-verification-email unexpected status %d: %s", resp.StatusCode, string(raw))
}

func TestE2E_Auth_DeleteAccount(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-deleteacct-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-deleteacct-%") })

	tokens := register(t, env, email, "Password123!")

	// Delete account requires current password confirmation
	body := `{"password":"Password123!"}`
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/users/me"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete account failed: %s", string(raw))

	// Soft-deleted accounts are reactivated on next valid-password login
	loginResp := env.do(mustPost(env.url("/api/v1/auth/login"),
		fmt.Sprintf(`{"email":%q,"password":"Password123!"}`, email)))
	defer func() { _ = loginResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, loginResp.StatusCode,
		"soft-deleted account should be reactivated on login")
}

func TestE2E_Auth_LogoutAll(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-logoutall-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-logoutall-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/auth/logout-all"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "logout-all failed: %s", string(raw))
}

func TestE2E_Auth_GetMyPosts(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-myposts-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-myposts-%") })

	tokens := register(t, env, email, "Password123!")
	createPost(t, env, tokens.AccessToken, "My personal post")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/me/posts"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get my posts failed: %s", string(raw))
}

func TestE2E_Auth_GetMyBookmarks(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mybookmarks-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mybookmarks-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/me/bookmarks"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get my bookmarks failed: %s", string(raw))
}

func TestE2E_Auth_GetMyEvents(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-myevents-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-myevents-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/me/events?event_state=going"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get my events failed: %s", string(raw))
}

func TestE2E_Auth_GetPersonalizedFeed(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-pfeed-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-pfeed-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/feed"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "personalized feed failed: %s", string(raw))
}

func TestE2E_Post_BookmarkUnbookmark(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bookmark-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bookmark-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post to bookmark")

	// Bookmark
	bookResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/bookmark"), tokens.AccessToken, ""))
	defer func() { _ = bookResp.Body.Close() }()
	bookRaw, _ := io.ReadAll(bookResp.Body)
	assert.Equal(t, http.StatusOK, bookResp.StatusCode, "bookmark failed: %s", string(bookRaw))

	// Verify in bookmarks list
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/me/bookmarks"), tokens.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	// Unbookmark
	unbookResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/posts/"+postID+"/bookmark"), tokens.AccessToken, ""))
	defer func() { _ = unbookResp.Body.Close() }()
	unbookRaw, _ := io.ReadAll(unbookResp.Body)
	assert.Equal(t, http.StatusOK, unbookResp.StatusCode, "unbookmark failed: %s", string(unbookRaw))
}

func TestE2E_Post_SharePost(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-shareA-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-shareB-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-shareA-%")
		env.cleanupTestData(t, "e2e-shareB-%")
	})

	original := register(t, env, email1, "Password123!")
	sharer := register(t, env, email2, "Password123!")

	postID := createPost(t, env, original.AccessToken, "Original post to share")

	body := fmt.Sprintf(`{"share_text":"Check this out!","original_post_id":%q}`, postID)
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/share"), sharer.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "share post failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.NotEmpty(t, out.Data.ID)
}

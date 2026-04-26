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

func TestE2E_HelpChat_SendAndGetMessages(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-helpchat-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-helpchat-%") })

	tokens := register(t, env, email, "Password123!")

	// Send a help message
	body := `{"content":"I need help with my account"}`
	sendResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/help-chat/messages"), tokens.AccessToken, body))
	defer func() { _ = sendResp.Body.Close() }()
	sendRaw, _ := io.ReadAll(sendResp.Body)
	assert.Equal(t, http.StatusCreated, sendResp.StatusCode, "send help message failed: %s", string(sendRaw))

	var sendOut struct {
		Data struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(sendRaw, &sendOut))
	assert.NotEmpty(t, sendOut.Data.ID)

	// Get messages
	getResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/help-chat/messages"), tokens.AccessToken, ""))
	defer func() { _ = getResp.Body.Close() }()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "get help messages failed: %s", string(getRaw))
}

func TestE2E_HelpChat_AdminCanSeeAndReplyToThreads(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	userEmail := fmt.Sprintf("e2e-helpchatuser-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-helpchatadmin-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-helpchatuser-%")
		env.cleanupTestData(t, "e2e-helpchatadmin-%")
	})

	user := register(t, env, userEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	// User sends help message
	body := `{"content":"Please help me with my post"}`
	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/help-chat/messages"), user.AccessToken, body)).Body.Close()

	// Admin gets all threads
	threadsResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/help-chat"), adminUser.AccessToken, ""))
	defer func() { _ = threadsResp.Body.Close() }()
	threadsRaw, _ := io.ReadAll(threadsResp.Body)
	assert.Equal(t, http.StatusOK, threadsResp.StatusCode,
		"admin get threads failed: %s", string(threadsRaw))

	// Admin gets specific user thread
	threadResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/help-chat/"+user.UserID), adminUser.AccessToken, ""))
	defer func() { _ = threadResp.Body.Close() }()
	threadRaw, _ := io.ReadAll(threadResp.Body)
	assert.Equal(t, http.StatusOK, threadResp.StatusCode,
		"admin get user thread failed: %s", string(threadRaw))

	// Admin replies
	replyBody := `{"content":"We are here to help you"}`
	replyResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/help-chat/"+user.UserID+"/reply"), adminUser.AccessToken, replyBody))
	defer func() { _ = replyResp.Body.Close() }()
	replyRaw, _ := io.ReadAll(replyResp.Body)
	assert.Equal(t, http.StatusCreated, replyResp.StatusCode,
		"admin reply failed: %s", string(replyRaw))
}

func TestE2E_HelpChat_NonAdminCannotAccessAdminEndpoints(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-helpchatnonadmin-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-helpchatnonadmin-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/help-chat"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

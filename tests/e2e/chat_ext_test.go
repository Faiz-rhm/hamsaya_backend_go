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

func TestE2E_Chat_MarkConversationAsRead(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-chatread1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-chatread2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-chatread1-%")
		env.cleanupTestData(t, "e2e-chatread2-%")
	})

	user1 := register(t, env, email1, "Password123!")
	user2 := register(t, env, email2, "Password123!")

	_, convID := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "Hello!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/chat/conversations/"+convID+"/read"), user2.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "mark conversation read failed: %s", string(raw))
}

func TestE2E_Chat_DeleteMessage(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-chatdelmsg1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-chatdelmsg2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-chatdelmsg1-%")
		env.cleanupTestData(t, "e2e-chatdelmsg2-%")
	})

	user1 := register(t, env, email1, "Password123!")
	user2 := register(t, env, email2, "Password123!")

	msgID, _ := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "Message to delete")

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/chat/messages/"+msgID), user1.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete message failed: %s", string(raw))
}

func TestE2E_Chat_DeleteMessageByNonSenderReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-chatdelauth1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-chatdelauth2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-chatdelauth1-%")
		env.cleanupTestData(t, "e2e-chatdelauth2-%")
	})

	user1 := register(t, env, email1, "Password123!")
	user2 := register(t, env, email2, "Password123!")

	msgID, _ := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "Message")

	// user2 tries to delete user1's message
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/chat/messages/"+msgID), user2.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestE2E_Chat_GetPollByID(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-pollbyid-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-pollbyid-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPullPost(t, env, tokens.AccessToken)

	// Get poll via post endpoint to retrieve poll ID
	pollResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/"+postID+"/polls"), tokens.AccessToken, ""))
	defer pollResp.Body.Close()
	pollRaw, _ := io.ReadAll(pollResp.Body)
	require.Equal(t, http.StatusOK, pollResp.StatusCode)

	var pollOut struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(pollRaw, &pollOut))
	pollID := pollOut.Data.ID
	require.NotEmpty(t, pollID)

	// Get poll directly by ID
	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/polls/"+pollID), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get poll by ID failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, pollID, out.Data.ID)
}

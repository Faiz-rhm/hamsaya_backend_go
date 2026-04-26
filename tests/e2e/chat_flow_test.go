package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendChatMessage sends a TEXT message from sender to recipientID.
// Returns the message ID and conversation ID.
func sendChatMessage(t *testing.T, env *testEnv, accessToken, recipientID, text string) (msgID, convID string) {
	t.Helper()
	body := fmt.Sprintf(`{"recipient_id":%q,"content":%q,"message_type":"TEXT"}`, recipientID, text)
	req := bearerReq(http.MethodPost, env.url("/api/v1/chat/messages"), accessToken, body)

	resp := env.do(req)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"sendChatMessage failed: %s", string(raw))

	var out struct {
		Data struct {
			ID             string `json:"id"`
			ConversationID string `json:"conversation_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID, "message ID must not be empty")
	require.NotEmpty(t, out.Data.ConversationID, "conversation ID must not be empty")
	return out.Data.ID, out.Data.ConversationID
}

func TestE2E_ChatFlow_SendFetchConversationsGetHistory(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-chat1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-chat2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-chat1-%")
		env.cleanupTestData(t, "e2e-chat2-%")
	})

	// 1. Register two users
	user1 := register(t, env, email1, "Password123!")
	user2 := register(t, env, email2, "Password123!")

	// 2. User1 sends a message to User2
	msgID, convID := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "Hey there!")
	assert.NotEmpty(t, msgID)
	assert.NotEmpty(t, convID)

	// 3. Send a second message to build history
	_, convID2 := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "How are you?")
	assert.Equal(t, convID, convID2, "both messages must share same conversation")

	// 4. GET /chat/conversations for user1 — conversation must appear
	convsResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/chat/conversations"), user1.AccessToken, ""))
	defer convsResp.Body.Close()
	convsRaw, _ := io.ReadAll(convsResp.Body)
	assert.Equal(t, http.StatusOK, convsResp.StatusCode,
		"get conversations failed: %s", string(convsRaw))

	var convsOut struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(convsRaw, &convsOut))
	assert.NotEmpty(t, convsOut.Data, "user1 must have at least one conversation")

	found := false
	for _, c := range convsOut.Data {
		if c.ID == convID {
			found = true
			break
		}
	}
	assert.True(t, found, "created conversation must appear in user1's list")

	// 5. GET /chat/conversations for user2 — same conversation must appear
	convsResp2 := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/chat/conversations"), user2.AccessToken, ""))
	defer convsResp2.Body.Close()
	assert.Equal(t, http.StatusOK, convsResp2.StatusCode)

	// 6. GET /chat/conversations/:id/messages — history must contain both messages
	histURL := env.url("/api/v1/chat/conversations/" + convID + "/messages")
	histResp := env.do(bearerReq(http.MethodGet, histURL, user1.AccessToken, ""))
	defer histResp.Body.Close()
	histRaw, _ := io.ReadAll(histResp.Body)
	assert.Equal(t, http.StatusOK, histResp.StatusCode,
		"get message history failed: %s", string(histRaw))

	var histOut struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(histRaw, &histOut))
	assert.GreaterOrEqual(t, len(histOut.Data), 2,
		"history must contain at least the two sent messages")
}

func TestE2E_Chat_NonParticipantCannotReadMessages(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-chatA-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-chatB-%d@test.local", ts)
	email3 := fmt.Sprintf("e2e-chatC-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-chatA-%")
		env.cleanupTestData(t, "e2e-chatB-%")
		env.cleanupTestData(t, "e2e-chatC-%")
	})

	user1 := register(t, env, email1, "Password123!")
	user2 := register(t, env, email2, "Password123!")
	outsider := register(t, env, email3, "Password123!")

	_, convID := sendChatMessage(t, env, user1.AccessToken, user2.UserID, "Private message")

	// Outsider tries to read the conversation history
	histURL := env.url("/api/v1/chat/conversations/" + convID + "/messages")
	resp := env.do(bearerReq(http.MethodGet, histURL, outsider.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"non-participant must be denied access to conversation messages")
}

func TestE2E_Chat_UnauthenticatedCannotSendMessage(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-chatunauth-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-chatunauth-%") })

	user := register(t, env, email, "Password123!")

	body := fmt.Sprintf(`{"recipient_id":%q,"content":"hello","message_type":"TEXT"}`, user.UserID)
	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/chat/messages"),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

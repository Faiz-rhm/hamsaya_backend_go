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

// createPullPost creates a PULL (poll) post with initial poll options.
// Returns the post ID.
func createPullPost(t *testing.T, env *testEnv, accessToken string) string {
	t.Helper()
	body := `{
		"type": "PULL",
		"description": "Which do you prefer?",
		"visibility": "PUBLIC",
		"poll_options": ["Option A", "Option B", "Option C"]
	}`
	resp := env.do(bearerReq(http.MethodPost, env.url("/api/v1/posts"), accessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createPullPost failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID)
	return out.Data.ID
}

func TestE2E_Poll_CreateAndGetPoll(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-poll-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-poll-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPullPost(t, env, tokens.AccessToken)

	// Get the poll for the post
	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/"+postID+"/polls"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get poll failed: %s", string(raw))

	var out struct {
		Data struct {
			Options []struct {
				ID     string `json:"id"`
				Option string `json:"option"`
			} `json:"options"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.GreaterOrEqual(t, len(out.Data.Options), 2, "poll must have at least 2 options")
}

func TestE2E_Poll_VoteOnPoll(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	creatorEmail := fmt.Sprintf("e2e-pollcreator-%d@test.local", ts)
	voterEmail := fmt.Sprintf("e2e-pollvoter-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-pollcreator-%")
		env.cleanupTestData(t, "e2e-pollvoter-%")
	})

	creator := register(t, env, creatorEmail, "Password123!")
	voter := register(t, env, voterEmail, "Password123!")

	postID := createPullPost(t, env, creator.AccessToken)

	// Get poll to find poll ID and option IDs
	pollResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/"+postID+"/polls"), voter.AccessToken, ""))
	defer func() { _ = pollResp.Body.Close() }()
	pollRaw, _ := io.ReadAll(pollResp.Body)
	require.Equal(t, http.StatusOK, pollResp.StatusCode)

	var pollOut struct {
		Data struct {
			ID      string `json:"id"`
			Options []struct {
				ID string `json:"id"`
			} `json:"options"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(pollRaw, &pollOut))
	require.NotEmpty(t, pollOut.Data.ID)
	require.NotEmpty(t, pollOut.Data.Options)

	optionID := pollOut.Data.Options[0].ID
	pollID := pollOut.Data.ID

	// Vote
	voteBody := fmt.Sprintf(`{"poll_option_id":%q}`, optionID)
	voteResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/polls/"+pollID+"/vote"), voter.AccessToken, voteBody))
	defer func() { _ = voteResp.Body.Close() }()
	voteRaw, _ := io.ReadAll(voteResp.Body)
	assert.Equal(t, http.StatusOK, voteResp.StatusCode, "vote failed: %s", string(voteRaw))

	// Delete vote
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/polls/"+pollID+"/vote"), voter.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "delete vote failed: %s", string(delRaw))
}

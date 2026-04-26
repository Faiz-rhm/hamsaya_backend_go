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

// createEventPost creates an EVENT post and returns its ID.
func createEventPost(t *testing.T, env *testEnv, accessToken string) string {
	t.Helper()
	startDate := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{
		"type": "EVENT",
		"title": "E2E Test Event",
		"description": "An event created in E2E tests",
		"visibility": "PUBLIC",
		"start_date": %q
	}`, startDate)
	resp := env.do(bearerReq(http.MethodPost, env.url("/api/v1/posts"), accessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createEventPost failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID)
	return out.Data.ID
}

func TestE2E_Event_SetGetRemoveInterest(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	creatorEmail := fmt.Sprintf("e2e-evtcreator-%d@test.local", ts)
	attendeeEmail := fmt.Sprintf("e2e-evtattendee-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-evtcreator-%")
		env.cleanupTestData(t, "e2e-evtattendee-%")
	})

	creator := register(t, env, creatorEmail, "Password123!")
	attendee := register(t, env, attendeeEmail, "Password123!")

	postID := createEventPost(t, env, creator.AccessToken)

	// Set interest
	setBody := `{"event_state":"interested"}`
	setResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/events/"+postID+"/interest"), attendee.AccessToken, setBody))
	defer func() { _ = setResp.Body.Close() }()
	setRaw, _ := io.ReadAll(setResp.Body)
	assert.Equal(t, http.StatusOK, setResp.StatusCode, "set interest failed: %s", string(setRaw))

	var setOut struct {
		Data struct {
			UserEventState  string `json:"user_event_state"`
			InterestedCount int    `json:"interested_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(setRaw, &setOut))
	assert.Equal(t, "interested", setOut.Data.UserEventState)
	assert.Equal(t, 1, setOut.Data.InterestedCount)

	// Get interest status
	getResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/events/"+postID+"/interest"), attendee.AccessToken, ""))
	defer func() { _ = getResp.Body.Close() }()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "get interest failed: %s", string(getRaw))

	// Get interested users list
	interestedResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/events/"+postID+"/interested"), attendee.AccessToken, ""))
	defer func() { _ = interestedResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, interestedResp.StatusCode)

	// Update to going
	goingBody := `{"event_state":"going"}`
	goingResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/events/"+postID+"/interest"), attendee.AccessToken, goingBody))
	defer func() { _ = goingResp.Body.Close() }()
	goingRaw, _ := io.ReadAll(goingResp.Body)
	assert.Equal(t, http.StatusOK, goingResp.StatusCode, "set going failed: %s", string(goingRaw))

	// Get going users list
	goingListResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/events/"+postID+"/going"), attendee.AccessToken, ""))
	defer func() { _ = goingListResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, goingListResp.StatusCode)

	// Remove interest
	removeResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/events/"+postID+"/interest"), attendee.AccessToken, ""))
	defer func() { _ = removeResp.Body.Close() }()
	removeRaw, _ := io.ReadAll(removeResp.Body)
	assert.Equal(t, http.StatusOK, removeResp.StatusCode, "remove interest failed: %s", string(removeRaw))
}

func TestE2E_Event_NonEventPostReturnsError(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-evtnonevent-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-evtnonevent-%") })

	tokens := register(t, env, email, "Password123!")
	feedPostID := createPost(t, env, tokens.AccessToken, "Not an event post")

	setBody := `{"event_state":"interested"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/events/"+feedPostID+"/interest"), tokens.AccessToken, setBody))
	defer func() { _ = resp.Body.Close() }()
	// Setting interest on a FEED post should fail
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

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

func TestE2E_Feedback_SubmitAndCheckStatus(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-feedback-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-feedback-%") })

	tokens := register(t, env, email, "Password123!")

	// Check status before submitting — should indicate no prior feedback
	statusResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/feedback/status"), tokens.AccessToken, ""))
	defer func() { _ = statusResp.Body.Close() }()
	statusRaw, _ := io.ReadAll(statusResp.Body)
	assert.Equal(t, http.StatusOK, statusResp.StatusCode, "get feedback status failed: %s", string(statusRaw))

	// Submit feedback
	body := `{
		"rating": 4,
		"type": "GENERAL",
		"message": "Great app, loving the E2E tests!"
	}`
	submitResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/feedback"), tokens.AccessToken, body))
	defer func() { _ = submitResp.Body.Close() }()
	submitRaw, _ := io.ReadAll(submitResp.Body)
	assert.Equal(t, http.StatusCreated, submitResp.StatusCode, "submit feedback failed: %s", string(submitRaw))

	var submitOut struct {
		Data struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitRaw, &submitOut))
	assert.NotEmpty(t, submitOut.Data.ID)

	// Check status after submitting
	statusResp2 := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/feedback/status"), tokens.AccessToken, ""))
	defer func() { _ = statusResp2.Body.Close() }()
	statusRaw2, _ := io.ReadAll(statusResp2.Body)
	assert.Equal(t, http.StatusOK, statusResp2.StatusCode, "get status after submit failed: %s", string(statusRaw2))
}

func TestE2E_Feedback_InvalidRatingReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-feedbackbad-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-feedbackbad-%") })

	tokens := register(t, env, email, "Password123!")

	body := `{"rating": 10, "type": "GENERAL", "message": "Too high rating"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/feedback"), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

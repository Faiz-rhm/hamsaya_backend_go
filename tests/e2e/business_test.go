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

// createBusiness creates a business profile and returns its ID.
func createBusiness(t *testing.T, env *testEnv, accessToken, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q}`, name)
	resp := env.do(bearerReq(http.MethodPost, env.url("/api/v1/businesses"), accessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createBusiness failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID)
	return out.Data.ID
}

func TestE2E_Business_CreateAndGet(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-biz-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-biz-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "E2E Test Biz")
	assert.NotEmpty(t, bizID)

	// Get the business
	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/businesses/"+bizID), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get business failed: %s", string(raw))

	var out struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, bizID, out.Data.ID)
	assert.Equal(t, "E2E Test Biz", out.Data.Name)
}

func TestE2E_Business_GetMyBusinesses(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-mybiz-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-mybiz-%") })

	tokens := register(t, env, email, "Password123!")
	createBusiness(t, env, tokens.AccessToken, "My Business")

	resp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/businesses"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get my businesses failed: %s", string(raw))
}

func TestE2E_Business_FollowUnfollow(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-bizowner-%d@test.local", ts)
	followerEmail := fmt.Sprintf("e2e-bizfollower-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-bizowner-%")
		env.cleanupTestData(t, "e2e-bizfollower-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	follower := register(t, env, followerEmail, "Password123!")

	bizID := createBusiness(t, env, owner.AccessToken, "Follow Test Biz")

	// Follow
	followResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/follow"), follower.AccessToken, ""))
	defer func() { _ = followResp.Body.Close() }()
	followRaw, _ := io.ReadAll(followResp.Body)
	assert.Equal(t, http.StatusOK, followResp.StatusCode, "follow business failed: %s", string(followRaw))

	// Unfollow
	unfollowResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/businesses/"+bizID+"/follow"), follower.AccessToken, ""))
	defer func() { _ = unfollowResp.Body.Close() }()
	unfollowRaw, _ := io.ReadAll(unfollowResp.Body)
	assert.Equal(t, http.StatusOK, unfollowResp.StatusCode, "unfollow business failed: %s", string(unfollowRaw))
}

func TestE2E_Business_DeleteBusiness(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizdel-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizdel-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Deletable Biz")

	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/businesses/"+bizID), tokens.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "delete business failed: %s", string(delRaw))
}

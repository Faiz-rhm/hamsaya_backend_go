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

func TestE2E_Relationships_FollowUnfollow(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-rel1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-rel2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-rel1-%")
		env.cleanupTestData(t, "e2e-rel2-%")
	})

	follower := register(t, env, email1, "Password123!")
	following := register(t, env, email2, "Password123!")

	// Follow
	followResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+following.UserID+"/follow"), follower.AccessToken, ""))
	defer followResp.Body.Close()
	followRaw, _ := io.ReadAll(followResp.Body)
	assert.Equal(t, http.StatusOK, followResp.StatusCode, "follow failed: %s", string(followRaw))

	// Check relationship status
	relResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/"+following.UserID+"/relationship"), follower.AccessToken, ""))
	defer relResp.Body.Close()
	relRaw, _ := io.ReadAll(relResp.Body)
	assert.Equal(t, http.StatusOK, relResp.StatusCode, "relationship status failed: %s", string(relRaw))

	var relOut struct {
		Data struct {
			IsFollowing bool `json:"is_following"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(relRaw, &relOut))
	assert.True(t, relOut.Data.IsFollowing)

	// Check followers list
	followersResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/"+following.UserID+"/followers"), follower.AccessToken, ""))
	defer followersResp.Body.Close()
	assert.Equal(t, http.StatusOK, followersResp.StatusCode)

	// Unfollow
	unfollowResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/users/"+following.UserID+"/follow"), follower.AccessToken, ""))
	defer unfollowResp.Body.Close()
	unfollowRaw, _ := io.ReadAll(unfollowResp.Body)
	assert.Equal(t, http.StatusOK, unfollowResp.StatusCode, "unfollow failed: %s", string(unfollowRaw))
}

func TestE2E_Relationships_GetFollowing(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-following1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-following2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-following1-%")
		env.cleanupTestData(t, "e2e-following2-%")
	})

	follower := register(t, env, email1, "Password123!")
	target := register(t, env, email2, "Password123!")

	// Follow
	env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+target.UserID+"/follow"), follower.AccessToken, "")).Body.Close()

	// Get following list
	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/"+follower.UserID+"/following"), follower.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get following failed: %s", string(raw))
}

func TestE2E_Relationships_BlockUnblock(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-block1-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-block2-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-block1-%")
		env.cleanupTestData(t, "e2e-block2-%")
	})

	blocker := register(t, env, email1, "Password123!")
	blocked := register(t, env, email2, "Password123!")

	// Block
	blockResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+blocked.UserID+"/block"), blocker.AccessToken, ""))
	defer blockResp.Body.Close()
	blockRaw, _ := io.ReadAll(blockResp.Body)
	assert.Equal(t, http.StatusOK, blockResp.StatusCode, "block failed: %s", string(blockRaw))

	// Get blocked list
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/blocked"), blocker.AccessToken, ""))
	defer listResp.Body.Close()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	// Unblock
	unblockResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/users/"+blocked.UserID+"/block"), blocker.AccessToken, ""))
	defer unblockResp.Body.Close()
	unblockRaw, _ := io.ReadAll(unblockResp.Body)
	assert.Equal(t, http.StatusOK, unblockResp.StatusCode, "unblock failed: %s", string(unblockRaw))
}

func TestE2E_Relationships_CannotFollowSelf(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-selffollow-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-selffollow-%") })

	user := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+user.UserID+"/follow"), user.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

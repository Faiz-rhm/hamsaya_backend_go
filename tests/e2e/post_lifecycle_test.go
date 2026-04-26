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

// createPost creates a FEED post and returns its ID.
func createPost(t *testing.T, env *testEnv, accessToken, description string) string {
	t.Helper()
	body := fmt.Sprintf(`{"type":"FEED","description":%q,"visibility":"PUBLIC"}`, description)
	req := bearerReq(http.MethodPost, env.url("/api/v1/posts"), accessToken, body)

	resp := env.do(req)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"createPost failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID, "post ID must not be empty")
	return out.Data.ID
}

func TestE2E_PostLifecycle_CreateFeedLikeCommentDelete(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-post-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-post-%") })

	// 1. Register user
	tokens := register(t, env, email, "Password123!")

	// 2. Create a FEED post
	postID := createPost(t, env, tokens.AccessToken, "Hello from E2E test")
	assert.NotEmpty(t, postID)

	// 3. GET /posts returns 200 (feed endpoint accessible)
	feedResp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/posts"), tokens.AccessToken, ""))
	defer func() { _ = feedResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, feedResp.StatusCode, "feed must be accessible")

	// 4. GET /posts/:post_id returns the post
	getResp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/posts/"+postID), tokens.AccessToken, ""))
	defer func() { _ = getResp.Body.Close() }()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode,
		"get post failed: %s", string(getRaw))

	var getOut struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRaw, &getOut))
	assert.Equal(t, postID, getOut.Data.ID)

	// 5. Like the post
	likeResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/like"), tokens.AccessToken, ""))
	defer func() { _ = likeResp.Body.Close() }()
	likeRaw, _ := io.ReadAll(likeResp.Body)
	assert.Equal(t, http.StatusOK, likeResp.StatusCode,
		"like post failed: %s", string(likeRaw))

	// 6. Create a comment
	commentBody := `{"text":"Great post!"}`
	commentResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/comments"), tokens.AccessToken, commentBody))
	defer func() { _ = commentResp.Body.Close() }()
	commentRaw, _ := io.ReadAll(commentResp.Body)
	assert.Equal(t, http.StatusCreated, commentResp.StatusCode,
		"create comment failed: %s", string(commentRaw))

	var commentOut struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(commentRaw, &commentOut))
	assert.NotEmpty(t, commentOut.Data.ID, "comment ID must not be empty")

	// 7. GET comments for the post
	commentsResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/"+postID+"/comments"), tokens.AccessToken, ""))
	defer func() { _ = commentsResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, commentsResp.StatusCode)

	// 8. Delete the post
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/posts/"+postID), tokens.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode,
		"delete post failed: %s", string(delRaw))

	// 9. GET after delete should return 404
	gone := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/posts/"+postID), tokens.AccessToken, ""))
	defer func() { _ = gone.Body.Close() }()
	assert.Equal(t, http.StatusNotFound, gone.StatusCode,
		"deleted post must return 404")
}

func TestE2E_Post_UnauthorizedLike(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-postauth-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-postauth-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for auth test")

	// Attempt like without token — must be rejected
	req, _ := http.NewRequest(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/like"), bytes.NewBufferString(""))
	resp := env.do(req)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_Post_DeleteByNonOwnerReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-owner-%d@test.local", ts)
	otherEmail := fmt.Sprintf("e2e-other-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-owner-%")
		env.cleanupTestData(t, "e2e-other-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	other := register(t, env, otherEmail, "Password123!")

	postID := createPost(t, env, owner.AccessToken, "Owner's post")

	// Other user tries to delete owner's post
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/posts/"+postID), other.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, delResp.StatusCode,
		"non-owner delete must be rejected")
}

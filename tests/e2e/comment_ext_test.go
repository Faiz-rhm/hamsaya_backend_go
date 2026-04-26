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

// createComment creates a comment and returns its ID.
func createComment(t *testing.T, env *testEnv, accessToken, postID, text string) string {
	t.Helper()
	body := fmt.Sprintf(`{"text":%q}`, text)
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/comments"), accessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "createComment failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Data.ID)
	return out.Data.ID
}

func TestE2E_Comment_LikeUnlikeDelete(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-commentext-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-commentext-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for comment tests")
	commentID := createComment(t, env, tokens.AccessToken, postID, "A test comment")

	// Like the comment
	likeResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/comments/"+commentID+"/like"), tokens.AccessToken, ""))
	defer likeResp.Body.Close()
	likeRaw, _ := io.ReadAll(likeResp.Body)
	assert.Equal(t, http.StatusOK, likeResp.StatusCode, "like comment failed: %s", string(likeRaw))

	// Unlike the comment
	unlikeResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/comments/"+commentID+"/like"), tokens.AccessToken, ""))
	defer unlikeResp.Body.Close()
	unlikeRaw, _ := io.ReadAll(unlikeResp.Body)
	assert.Equal(t, http.StatusOK, unlikeResp.StatusCode, "unlike comment failed: %s", string(unlikeRaw))

	// Get comment
	getResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/comments/"+commentID), tokens.AccessToken, ""))
	defer getResp.Body.Close()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "get comment failed: %s", string(getRaw))

	// Get replies (empty for a top-level comment)
	repliesResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/comments/"+commentID+"/replies"), tokens.AccessToken, ""))
	defer repliesResp.Body.Close()
	assert.Equal(t, http.StatusOK, repliesResp.StatusCode)

	// Delete the comment
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/comments/"+commentID), tokens.AccessToken, ""))
	defer delResp.Body.Close()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "delete comment failed: %s", string(delRaw))
}

func TestE2E_Comment_ReplyToComment(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-commentreply-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-commentreply-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for reply tests")
	parentID := createComment(t, env, tokens.AccessToken, postID, "Parent comment")

	// Create a reply
	replyBody := fmt.Sprintf(`{"text":"A reply","parent_comment_id":%q}`, parentID)
	replyResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/comments"), tokens.AccessToken, replyBody))
	defer replyResp.Body.Close()
	replyRaw, _ := io.ReadAll(replyResp.Body)
	assert.Equal(t, http.StatusCreated, replyResp.StatusCode, "create reply failed: %s", string(replyRaw))

	// Verify reply appears in replies list
	repliesResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/comments/"+parentID+"/replies"), tokens.AccessToken, ""))
	defer repliesResp.Body.Close()
	repliesRaw, _ := io.ReadAll(repliesResp.Body)
	assert.Equal(t, http.StatusOK, repliesResp.StatusCode, "get replies failed: %s", string(repliesRaw))
}

func TestE2E_Post_UnlikePost(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-unlike-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-unlike-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for unlike test")

	// Like first
	env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/like"), tokens.AccessToken, "")).Body.Close()

	// Unlike
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/posts/"+postID+"/like"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "unlike post failed: %s", string(raw))
}

func TestE2E_Comment_DeleteByNonOwnerReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-cmtowner-%d@test.local", ts)
	otherEmail := fmt.Sprintf("e2e-cmtother-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-cmtowner-%")
		env.cleanupTestData(t, "e2e-cmtother-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	other := register(t, env, otherEmail, "Password123!")

	postID := createPost(t, env, owner.AccessToken, "Post for auth test")
	commentID := createComment(t, env, owner.AccessToken, postID, "Owner's comment")

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/comments/"+commentID), other.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestE2E_Comment_UpdateComment(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-cmtupd-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-cmtupd-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for comment update")
	commentID := createComment(t, env, tokens.AccessToken, postID, "Original text")

	body := `{"text":"Updated comment text"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/comments/"+commentID), tokens.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update comment failed: %s", string(raw))
}

func TestE2E_Comment_UpdateByNonOwnerReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-cmtupdowner-%d@test.local", ts)
	otherEmail := fmt.Sprintf("e2e-cmtupdother-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-cmtupdowner-%")
		env.cleanupTestData(t, "e2e-cmtupdother-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	other := register(t, env, otherEmail, "Password123!")

	postID := createPost(t, env, owner.AccessToken, "Post")
	commentID := createComment(t, env, owner.AccessToken, postID, "Owner comment")

	body := `{"text":"Trying to edit someone else's comment"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/comments/"+commentID), other.AccessToken, body))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

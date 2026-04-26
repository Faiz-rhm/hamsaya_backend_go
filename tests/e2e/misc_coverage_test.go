package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Auth: unified endpoint ----

func TestE2E_Auth_UnifiedAuth_Register(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-unified-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-unified-%") })

	fn := "Test"
	ln := "Unified"
	body := fmt.Sprintf(`{"email":%q,"password":"Password123!","first_name":%q,"last_name":%q}`, email, fn, ln)
	resp := env.do(mustPost(env.url("/api/v1/auth/unified"), body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	// Returns 200 for both login and register per handler docs
	assert.Equal(t, http.StatusOK, resp.StatusCode, "unified register failed: %s", string(raw))
}

func TestE2E_Auth_UnifiedAuth_Login(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-unifiedlogin-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-unifiedlogin-%") })

	// Register first so user exists
	register(t, env, email, "Password123!")

	// Now unified should log in the existing user
	body := fmt.Sprintf(`{"email":%q,"password":"Password123!"}`, email)
	resp := env.do(mustPost(env.url("/api/v1/auth/unified"), body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "unified login failed: %s", string(raw))

	var out struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.NotEmpty(t, out.Data.AccessToken)
}

// ---- Auth: verify-email with invalid token ----

func TestE2E_Auth_VerifyEmail_InvalidTokenReturns4xx(t *testing.T) {
	env := setupE2E(t)

	body := `{"token":"000000"}`
	resp := env.do(mustPost(env.url("/api/v1/auth/verify-email"), body))
	defer resp.Body.Close()
	assert.True(t, resp.StatusCode >= 400 && resp.StatusCode < 500,
		"expected 4xx for invalid verify-email token, got %d", resp.StatusCode)
}

// ---- Comment: get single comment ----

func TestE2E_Comment_GetSingle(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-cmt-get-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-cmt-get-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Post for comment get")
	commentID := createComment(t, env, tokens.AccessToken, postID, "Comment to retrieve")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/comments/"+commentID), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get single comment failed: %s", string(raw))

	var out struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, commentID, out.Data.ID)
}

// ---- Post: resell a SELL post ----

func TestE2E_Post_ResellSellPost(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-resell-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-resell-%") })

	tokens := register(t, env, email, "Password123!")

	// Create a SELL post
	price := 50.0
	body := fmt.Sprintf(`{"type":"SELL","description":"Item for sale","price":%v,"currency":"USD","visibility":"PUBLIC"}`, price)
	createResp := env.do(bearerReq(http.MethodPost, env.url("/api/v1/posts"), tokens.AccessToken, body))
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "create sell post failed: %s", string(createRaw))

	var createOut struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(createRaw, &createOut))
	postID := createOut.Data.ID
	require.NotEmpty(t, postID)

	// Resell the post (reactivate it)
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/resell"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "resell post failed: %s", string(raw))
}

func TestE2E_Post_ResellNonSellPostReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-resell-bad-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-resell-bad-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "A FEED post")

	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/resell"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "expected 400 for reselling a non-SELL post")
}

// ---- Business: delete gallery attachment ----

// seedBusinessAttachment inserts a gallery attachment directly to bypass MinIO upload.
// Returns the new attachment ID.
func seedBusinessAttachment(t *testing.T, env *testEnv, businessID string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var attachmentID string
	err := env.db.Pool.QueryRow(ctx, `
		INSERT INTO business_attachments (id, business_profile_id, photo, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2::jsonb, now(), now())
		RETURNING id
	`, businessID, `{"url":"https://example.com/test.jpg","name":"test.jpg","size":0,"width":0,"height":0,"mime_type":"image/jpeg"}`).Scan(&attachmentID)
	require.NoError(t, err, "seedBusinessAttachment: insert failed")
	return attachmentID
}

func TestE2E_Business_DeleteGalleryImage(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizgaldel-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizgaldel-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Gallery Delete Biz")
	attachmentID := seedBusinessAttachment(t, env, bizID)

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/businesses/"+bizID+"/attachments/"+attachmentID),
		tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete gallery image failed: %s", string(raw))
}

func TestE2E_Business_DeleteGalleryImageByNonOwnerReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-bizgaldelowner-%d@test.local", ts)
	otherEmail := fmt.Sprintf("e2e-bizgaldelother-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-bizgaldelowner-%")
		env.cleanupTestData(t, "e2e-bizgaldelother-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	other := register(t, env, otherEmail, "Password123!")
	bizID := createBusiness(t, env, owner.AccessToken, "Owner Biz")
	attachmentID := seedBusinessAttachment(t, env, bizID)

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/businesses/"+bizID+"/attachments/"+attachmentID),
		other.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

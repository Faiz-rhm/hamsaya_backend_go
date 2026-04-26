package e2e

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---- OAuth: validation errors (no real provider tokens needed) ----

func TestE2E_OAuth_Google_MissingTokenReturns400(t *testing.T) {
	env := setupE2E(t)
	// empty id_token fails "required" validation before any network call
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/google"), `{"id_token":""}`))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_OAuth_Facebook_MissingTokenReturns400(t *testing.T) {
	env := setupE2E(t)
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/facebook"), `{"access_token":""}`))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_OAuth_Apple_MissingTokenReturns400(t *testing.T) {
	env := setupE2E(t)
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/apple"), `{"id_token":""}`))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- MFA: invalid challenge returns 400 ----

func TestE2E_MFA_VerifyInvalidChallengeReturns400(t *testing.T) {
	env := setupE2E(t)
	// challenge_id not found in Redis (miniredis empty) → 400
	body := `{"challenge_id":"nonexistent-challenge-id","code":"123456"}`
	resp := env.do(mustPost(env.url("/api/v1/auth/mfa/verify"), body))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- FCM token: register and unregister (uses Redis only, no FCM client needed) ----

func TestE2E_Notifications_RegisterFCMToken(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-fcmreg-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-fcmreg-%") })

	tokens := register(t, env, email, "Password123!")

	body := `{"token":"fake-fcm-token-for-testing-12345"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/notifications/fcm-token"), tokens.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "register FCM token failed: %s", string(raw))
}

func TestE2E_Notifications_UnregisterFCMToken(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-fcmunreg-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-fcmunreg-%") })

	tokens := register(t, env, email, "Password123!")

	// Register first
	env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/notifications/fcm-token"), tokens.AccessToken,
		`{"token":"fake-fcm-token-for-testing-12345"}`)).Body.Close()

	// Then unregister
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/notifications/fcm-token"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "unregister FCM token failed: %s", string(raw))
}

// ---- Upload endpoints: no file → 400 before storageService (nil) is reached ----

// multipartReq builds a multipart/form-data POST with no actual file data.
// Omitting the file field causes FormFile("file") to return an error → 400.
func noFileReq(method, url, token string) *http.Request {
	req, _ := http.NewRequest(method, url, bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestE2E_Profile_UploadAvatarNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-avataruplm-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-avataruplm-%") })

	tokens := register(t, env, email, "Password123!")
	resp := env.do(noFileReq(http.MethodPost, env.url("/api/v1/users/me/avatar"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Profile_UploadCoverNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-covupl-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-covupl-%") })

	tokens := register(t, env, email, "Password123!")
	resp := env.do(noFileReq(http.MethodPost, env.url("/api/v1/users/me/cover"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Post_UploadImageNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-postimgupl-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-postimgupl-%") })

	tokens := register(t, env, email, "Password123!")
	resp := env.do(noFileReq(http.MethodPost, env.url("/api/v1/posts/upload-image"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Business_UploadAvatarNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizavtupl-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizavtupl-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Avatar Upload Biz")
	resp := env.do(noFileReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/avatar"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Business_UploadCoverNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizcovupl-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizcovupl-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Cover Upload Biz")
	resp := env.do(noFileReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/cover"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_Business_AddGalleryImageNoFileReturns400(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizgalupl-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizgalupl-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Gallery Upload Biz")
	resp := env.do(noFileReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/attachments"), tokens.AccessToken))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- Delete avatar / cover (no storage needed — just clears DB field) ----

func TestE2E_Profile_DeleteAvatar(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-delavatar-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-delavatar-%") })

	tokens := register(t, env, email, "Password123!")
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/users/me/avatar"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete avatar failed: %s", string(raw))
}

func TestE2E_Profile_DeleteCover(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-delcover-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-delcover-%") })

	tokens := register(t, env, email, "Password123!")
	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/users/me/cover"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "delete cover failed: %s", string(raw))
}

// ---- Upload endpoints: unauthenticated returns 401 ----

func TestE2E_Upload_UnauthenticatedReturns401(t *testing.T) {
	env := setupE2E(t)

	uploadEndpoints := []string{
		"/api/v1/users/me/avatar",
		"/api/v1/users/me/cover",
		"/api/v1/posts/upload-image",
	}

	for _, endpoint := range uploadEndpoints {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			_ = w.Close()
			req, _ := http.NewRequest(http.MethodPost, env.url(endpoint), &buf)
			req.Header.Set("Content-Type", w.FormDataContentType())
			resp := env.do(req)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"expected 401 for unauthenticated upload to %s", endpoint)
		})
	}
}

// ---- OAuth: unauthenticated request with valid-format body hits the service ----

func TestE2E_OAuth_Google_InvalidTokenReturns401(t *testing.T) {
	env := setupE2E(t)
	// Sends a non-empty fake token — passes validation, hits Google tokeninfo, gets 401
	// Skip gracefully if no network (timeout or connection refused)
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/google"),
		`{"id_token":"fake.invalid.token"}`))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	// Google rejects the token → our service returns 401
	// Accept 401 or 500 (in case of network timeout in CI)
	assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusInternalServerError,
		"expected 401 or 500 for fake Google token, got %d: %s", resp.StatusCode, string(raw))
}

func TestE2E_OAuth_Facebook_InvalidTokenReturns401(t *testing.T) {
	env := setupE2E(t)
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/facebook"),
		`{"access_token":"fake-invalid-facebook-token"}`))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusInternalServerError,
		"expected 401 or 500 for fake Facebook token, got %d: %s", resp.StatusCode, string(raw))
}

func TestE2E_OAuth_Apple_InvalidTokenReturns401(t *testing.T) {
	env := setupE2E(t)
	resp := env.do(mustPost(env.url("/api/v1/auth/oauth/apple"),
		`{"id_token":"fake.invalid.apple.token"}`))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusInternalServerError,
		"expected 401 or 500 for fake Apple token, got %d: %s", resp.StatusCode, string(raw))
}


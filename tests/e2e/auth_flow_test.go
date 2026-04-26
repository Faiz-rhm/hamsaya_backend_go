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

// authTokens holds the tokens returned after a successful auth call.
type authTokens struct {
	AccessToken  string
	RefreshToken string
	UserID       string
}

// register creates a new user and returns their tokens.
func register(t *testing.T, env *testEnv, email, password string) authTokens {
	t.Helper()
	body := fmt.Sprintf(`{
		"email": %q,
		"password": %q,
		"first_name": "Test",
		"last_name": "User",
		"latitude": 34.5553,
		"longitude": 69.2075
	}`, email, password)

	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/register"),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := env.do(req)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"register failed: %s", string(raw))

	var out struct {
		Data struct {
			Tokens *struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
			User *struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotNil(t, out.Data.Tokens, "no tokens in register response")
	return authTokens{
		AccessToken:  out.Data.Tokens.AccessToken,
		RefreshToken: out.Data.Tokens.RefreshToken,
		UserID:       out.Data.User.ID,
	}
}

// login authenticates an existing user.
func login(t *testing.T, env *testEnv, email, password string) authTokens {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)

	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/login"),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := env.do(req)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"login failed: %s", string(raw))

	var out struct {
		Data struct {
			Tokens *struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			} `json:"tokens"`
			User *struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotNil(t, out.Data.Tokens, "no tokens in login response: %s", string(raw))
	return authTokens{
		AccessToken:  out.Data.Tokens.AccessToken,
		RefreshToken: out.Data.Tokens.RefreshToken,
		UserID:       out.Data.User.ID,
	}
}

// bearerReq creates a request with an Authorization: Bearer header.
func bearerReq(method, url, accessToken, body string) *http.Request {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, url, bodyReader)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// --- Tests ---

func TestE2E_AuthFlow_RegisterLoginRefreshLogout(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-auth-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-auth-%") })

	// 1. Register
	tokens := register(t, env, email, "Password123!")
	assert.NotEmpty(t, tokens.AccessToken, "access token must not be empty")
	assert.NotEmpty(t, tokens.RefreshToken, "refresh token must not be empty")
	assert.NotEmpty(t, tokens.UserID, "user ID must not be empty")

	// 2. Login with same credentials
	loginTokens := login(t, env, email, "Password123!")
	assert.NotEmpty(t, loginTokens.AccessToken)

	// 3. Refresh token
	refreshBody := fmt.Sprintf(`{"refresh_token":%q}`, loginTokens.RefreshToken)
	refreshReq, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/refresh"),
		bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")

	refreshResp := env.do(refreshReq)
	defer refreshResp.Body.Close()
	refreshRaw, _ := io.ReadAll(refreshResp.Body)
	assert.Equal(t, http.StatusOK, refreshResp.StatusCode,
		"refresh failed: %s", string(refreshRaw))

	var refreshOut struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(refreshRaw, &refreshOut))
	newAccessToken := refreshOut.Data.AccessToken
	assert.NotEmpty(t, newAccessToken, "new access token must not be empty")

	// 4. Logout with the new access token
	logoutResp := env.do(bearerReq(http.MethodPost, env.url("/api/v1/auth/logout"), newAccessToken, ""))
	defer logoutResp.Body.Close()
	assert.Equal(t, http.StatusOK, logoutResp.StatusCode)

	// 5. After logout, the token should be rejected
	time.Sleep(50 * time.Millisecond) // let token blacklist propagate
	checkResp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/posts"), newAccessToken, ""))
	defer checkResp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, checkResp.StatusCode,
		"revoked token must be rejected")
}

func TestE2E_Auth_WrongPasswordReturns401(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-wrongpw-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-wrongpw-%") })

	register(t, env, email, "Password123!")

	body := fmt.Sprintf(`{"email":%q,"password":"WrongPass999!"}`, email)
	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/login"),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestE2E_Auth_DuplicateRegisterReturns409(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-dup-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-dup-%") })

	register(t, env, email, "Password123!")

	// Second registration with same email
	body := fmt.Sprintf(`{
		"email": %q,
		"password": "Password123!",
		"first_name": "Test",
		"last_name": "User",
		"latitude": 34.5553,
		"longitude": 69.2075
	}`, email)
	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/register"),
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestE2E_Auth_UnauthenticatedRequestReturns401(t *testing.T) {
	env := setupE2E(t)
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/posts"), nil)
	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

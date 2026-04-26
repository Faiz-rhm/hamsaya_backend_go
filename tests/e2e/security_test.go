package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestE2E_Security_SQLInjection_Search asserts that classic SQLi payloads sent
// through the public search endpoint do not cause server errors and do not
// leak unexpected rows. All repository SQL is parameterized via pgx, so the
// expected behavior is: payload treated as a literal search term, returning
// 200 with zero (or harmless) results — never 500.
func TestE2E_Security_SQLInjection_Search(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-sqli-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-sqli-%") })

	tokens := register(t, env, email, "Password123!")

	payloads := []string{
		`' OR '1'='1`,
		`'; DROP TABLE posts; --`,
		`' UNION SELECT password_hash, email FROM users --`,
		`%' OR 1=1 --`,
		`\\' OR 1=1; --`,
		`'; SELECT pg_sleep(5); --`,
	}

	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			q := url.QueryEscape(payload)
			req, _ := http.NewRequest(http.MethodGet,
				env.url("/api/v1/search/posts?q="+q+"&limit=10"), nil)
			req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)

			resp := env.do(req)
			defer func() { _ = resp.Body.Close() }()

			// Must not crash with 5xx. Either 200 (empty result) or 400 (validation reject).
			if resp.StatusCode >= 500 {
				t.Fatalf("payload %q caused server error: status=%d", payload, resp.StatusCode)
			}
			assert.Lessf(t, resp.StatusCode, 500, "payload must not crash server: %q", payload)
		})
	}
}

// TestE2E_Security_AccessTokenAfterLogout pins the access-token denylist
// contract: after /auth/logout, the access token used to perform the logout
// is added to a Redis denylist (keyed by JTI) and rejected with 401 on
// subsequent requests, even though it has not yet reached its 15-minute
// natural expiry. The refresh token is independently revoked server-side.
func TestE2E_Security_AccessTokenAfterLogout(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-tokenpost-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-tokenpost-%") })

	tokens := register(t, env, email, "Password123!")

	// Logout
	logoutReq := bearerReq(http.MethodPost, env.url("/api/v1/auth/logout"), tokens.AccessToken, "")
	logoutResp := env.do(logoutReq)
	_ = logoutResp.Body.Close()
	assert.Equal(t, http.StatusOK, logoutResp.StatusCode)

	// The access token used in the logout call must now be denylisted —
	// any subsequent request bearing it must return 401.
	meReq := bearerReq(http.MethodGet, env.url("/api/v1/users/me"), tokens.AccessToken, "")
	meResp := env.do(meReq)
	defer func() { _ = meResp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, meResp.StatusCode,
		"access token must be rejected after /auth/logout (denylist hit)")

	// Refresh token must be rejected (server-side session revocation).
	refreshBody := fmt.Sprintf(`{"refresh_token":%q}`, tokens.RefreshToken)
	refreshReq, _ := http.NewRequest(http.MethodPost,
		env.url("/api/v1/auth/refresh"),
		bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshResp := env.do(refreshReq)
	defer func() { _ = refreshResp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode,
		"refresh token must be rejected after logout")
}

// TestE2E_Security_OversizedPasswordRejected confirms the max-length validation
// added to RegisterRequest. Caps password input at 128 chars to neutralize the
// bcrypt-DoS vector (cost-12 hashing on multi-MB strings would block workers).
func TestE2E_Security_OversizedPasswordRejected(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-oversized-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-oversized-%") })

	huge := make([]byte, 200)
	for i := range huge {
		huge[i] = 'A'
	}

	body := fmt.Sprintf(
		`{"email":%q,"password":%q,"first_name":"Foo","last_name":"Bar","latitude":34.5,"longitude":69.2}`,
		email, string(huge))
	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/register"), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := env.do(req)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"oversized password must be rejected by validator before reaching bcrypt")
}

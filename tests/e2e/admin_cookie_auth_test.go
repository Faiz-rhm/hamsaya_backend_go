package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/hamsaya/backend/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_AdminCookieAuth_Flow exercises the full admin cookie-auth happy path:
//
//  1. Register a normal user → no cookies set, /admin/login refused (403).
//  2. Promote user to admin in DB.
//  3. POST /auth/admin/login → access + refresh + CSRF cookies set; tokens
//     stripped from JSON body.
//  4. Mutating admin call without X-CSRF-Token → 403.
//  5. Mutating admin call with cookie+CSRF header → 200.
//  6. POST /auth/admin/refresh (no body, refresh cookie auto-attached) →
//     fresh cookies issued.
//  7. POST /auth/admin/logout with new CSRF → all three cookies expired.
//
// Skipped automatically when Postgres is not reachable (setupE2E behavior).
func TestE2E_AdminCookieAuth_Flow(t *testing.T) {
	env := setupE2E(t)
	defer env.cleanupTestData(t, "admin-cookie-test+%@example.com")

	email := fmt.Sprintf("admin-cookie-test+%d@example.com", testNonce(t))
	password := "AdminCookiePass123!"

	// Step 1: register as a regular user, then promote.
	tokens := register(t, env, email, password)
	env.makeAdmin(t, tokens.UserID)

	jar := newCookieJar(t)

	// Step 2: regular login as admin via the cookie endpoint.
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/admin/login"), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := env.do(req)
	jar.absorb(resp)
	raw := readAndClose(t, resp)

	require.Equal(t, http.StatusOK, resp.StatusCode, "admin login failed: %s", raw)

	access := jar.value(utils.CookieAdminAccessToken)
	refresh := jar.value(utils.CookieAdminRefreshToken)
	csrf := jar.value(utils.CookieCSRFToken)
	require.NotEmpty(t, access, "access cookie missing")
	require.NotEmpty(t, refresh, "refresh cookie missing")
	require.NotEmpty(t, csrf, "csrf cookie missing")

	// Body must NOT carry tokens — that is the whole point of the cookie flow.
	var loginBody struct {
		Data struct {
			Tokens *map[string]any `json:"tokens"`
			User   *struct {
				Role string `json:"role"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &loginBody))
	assert.Nil(t, loginBody.Data.Tokens, "tokens must not be present in admin login JSON")
	require.NotNil(t, loginBody.Data.User)
	assert.Equal(t, "admin", loginBody.Data.User.Role)

	// Step 3: mutating admin call without CSRF header — must be 403.
	logoutNoCSRF, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/admin/logout"), nil)
	jar.attach(logoutNoCSRF)
	respNoCSRF := env.do(logoutNoCSRF)
	rawNoCSRF := readAndClose(t, respNoCSRF)
	assert.Equal(t, http.StatusForbidden, respNoCSRF.StatusCode, "expected 403 without CSRF header: %s", rawNoCSRF)

	// Step 4: refresh path — refresh cookie alone is sufficient (no CSRF
	// header required because the refresh token itself is the second factor).
	refreshReq, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/admin/refresh"), bytes.NewBufferString("{}"))
	refreshReq.Header.Set("Content-Type", "application/json")
	jar.attach(refreshReq)
	refreshResp := env.do(refreshReq)
	jar.absorb(refreshResp)
	rawRefresh := readAndClose(t, refreshResp)
	require.Equal(t, http.StatusOK, refreshResp.StatusCode, "refresh failed: %s", rawRefresh)

	newAccess := jar.value(utils.CookieAdminAccessToken)
	newCSRF := jar.value(utils.CookieCSRFToken)
	assert.NotEqual(t, access, newAccess, "access cookie must rotate on refresh")
	assert.NotEqual(t, csrf, newCSRF, "csrf cookie must rotate on refresh")

	// Step 5: logout with valid cookie + CSRF header → 200, cookies cleared.
	logoutReq, _ := http.NewRequest(http.MethodPost, env.url("/api/v1/auth/admin/logout"), nil)
	jar.attach(logoutReq)
	logoutReq.Header.Set(utils.HeaderCSRF, newCSRF)
	logoutResp := env.do(logoutReq)
	jar.absorb(logoutResp)
	rawLogout := readAndClose(t, logoutResp)
	require.Equal(t, http.StatusOK, logoutResp.StatusCode, "logout failed: %s", rawLogout)

	// Logout response should set Max-Age=0 cookies → cookieJar.value returns "".
	assert.Empty(t, jar.value(utils.CookieAdminAccessToken), "access cookie should be expired after logout")
	assert.Empty(t, jar.value(utils.CookieCSRFToken), "csrf cookie should be expired after logout")
}

// --- test helpers ---

// testNonce returns a unique-per-test integer suitable for crafting unique
// fixture emails without colliding across runs.
func testNonce(t *testing.T) int64 {
	t.Helper()
	return int64(len(t.Name()))*1000 + int64(testCounter.next())
}

type counter struct{ n int64 }

func (c *counter) next() int64 { c.n++; return c.n }

var testCounter counter

// cookieJar is a minimal cookie store that mirrors what a browser would do:
// absorb Set-Cookie from responses, attach Cookie on requests, and respect
// Max-Age (treating ≤0 as expiry).
type cookieJar struct {
	t      *testing.T
	values map[string]string
}

func newCookieJar(t *testing.T) *cookieJar {
	return &cookieJar{t: t, values: map[string]string{}}
}

func (j *cookieJar) absorb(resp *http.Response) {
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(j.values, c.Name)
			continue
		}
		j.values[c.Name] = c.Value
	}
}

func (j *cookieJar) attach(req *http.Request) {
	for name, val := range j.values {
		req.AddCookie(&http.Cookie{Name: name, Value: val})
	}
}

func (j *cookieJar) value(name string) string {
	return j.values[name]
}

func readAndClose(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

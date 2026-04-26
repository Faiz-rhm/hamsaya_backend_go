package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestE2E_Search_Users(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-search-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-search-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search/users?query=Test"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Posts(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-searchpost-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-searchpost-%") })

	tokens := register(t, env, email, "Password123!")
	createPost(t, env, tokens.AccessToken, "Searchable post content")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search/posts?query=Searchable"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Businesses(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-searchbiz-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-searchbiz-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search/businesses?query=Test"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Discover(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-discover-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-discover-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/discover?latitude=34.5553&longitude=69.2075&radius_km=10"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Unauthenticated(t *testing.T) {
	env := setupE2E(t)
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/search/users?query=test"), nil)
	resp := env.do(req)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

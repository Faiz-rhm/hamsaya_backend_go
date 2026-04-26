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
		env.url("/api/v1/search/users?q=Test"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Posts(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-searchpost-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-searchpost-%") })

	tokens := register(t, env, email, "Password123!")
	createPost(t, env, tokens.AccessToken, "Searchable post content")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search/posts?q=Searchable"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Businesses(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-searchbiz-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-searchbiz-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search/businesses?q=Test"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Discover(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-discover-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-discover-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/discover"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_Search_Unauthenticated(t *testing.T) {
	env := setupE2E(t)
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/search/users?q=test"), nil)
	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

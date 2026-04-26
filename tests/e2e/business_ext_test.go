package e2e

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestE2E_Business_UpdateBusiness(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizupd-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizupd-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Original Name")

	body := `{"name":"Updated Name","description":"New description"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/businesses/"+bizID), tokens.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update business failed: %s", string(raw))
}

func TestE2E_Business_SearchBusinesses(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizsearch-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizsearch-%") })

	tokens := register(t, env, email, "Password123!")
	createBusiness(t, env, tokens.AccessToken, "Searchable Business")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/businesses/search?q=Searchable"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "search businesses failed: %s", string(raw))
}

func TestE2E_Business_GetCategories(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizcategories-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizcategories-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/businesses/categories"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get categories failed: %s", string(raw))
}

func TestE2E_Business_GetHours(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizhours-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizhours-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Hours Test Biz")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/businesses/"+bizID+"/hours"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get hours failed: %s", string(raw))
}

func TestE2E_Business_GetGallery(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizgallery-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizgallery-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Gallery Test Biz")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/businesses/"+bizID+"/attachments"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get gallery failed: %s", string(raw))
}

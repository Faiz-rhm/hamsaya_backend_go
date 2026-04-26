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

func TestE2E_Profile_GetMyProfile(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-profile-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-profile-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet, env.url("/api/v1/users/me"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "get profile failed: %s", string(raw))

	var out struct {
		Data struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, tokens.UserID, out.Data.ID)
	assert.Equal(t, email, out.Data.Email)
	assert.Equal(t, "Test", out.Data.FirstName)
}

func TestE2E_Profile_UpdateProfile(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-profileupd-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-profileupd-%") })

	tokens := register(t, env, email, "Password123!")

	body := `{"first_name":"Updated","last_name":"Name","bio":"E2E test bio"}`
	resp := env.do(bearerReq(http.MethodPut, env.url("/api/v1/users/me"), tokens.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "update profile failed: %s", string(raw))

	var out struct {
		Data struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "Updated", out.Data.FirstName)
	assert.Equal(t, "Name", out.Data.LastName)
}

func TestE2E_Profile_GetOtherUserProfile(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("e2e-profA-%d@test.local", ts)
	email2 := fmt.Sprintf("e2e-profB-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-profA-%")
		env.cleanupTestData(t, "e2e-profB-%")
	})

	viewer := register(t, env, email1, "Password123!")
	target := register(t, env, email2, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/users/"+target.UserID), viewer.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "get other profile failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, target.UserID, out.Data.ID)
}

func TestE2E_Profile_GetMyProfile_Unauthenticated(t *testing.T) {
	env := setupE2E(t)
	req, _ := http.NewRequest(http.MethodGet, env.url("/api/v1/users/me"), nil)
	resp := env.do(req)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

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

func TestE2E_Admin_GetStats(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-adminstats-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-adminstats-%") })

	adminUser := register(t, env, email, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/stats"), adminUser.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "admin stats failed: %s", string(raw))

	var out struct {
		Data struct {
			TotalUsers int64 `json:"total_users"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.GreaterOrEqual(t, out.Data.TotalUsers, int64(1))
}

func TestE2E_Admin_ListUsers(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-adminlist-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-adminlist-%") })

	adminUser := register(t, env, email, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/users"), adminUser.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "admin list users failed: %s", string(raw))
}

func TestE2E_Admin_GetUser(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	targetEmail := fmt.Sprintf("e2e-admintarget-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-admingetuser-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-admintarget-%")
		env.cleanupTestData(t, "e2e-admingetuser-%")
	})

	target := register(t, env, targetEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/users/"+target.UserID), adminUser.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "admin get user failed: %s", string(raw))

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, target.UserID, out.Data.ID)
}

func TestE2E_Admin_SuspendAndUnsuspendUser(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	targetEmail := fmt.Sprintf("e2e-adminsuspend-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-adminsuspendadmin-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-adminsuspend-%")
		env.cleanupTestData(t, "e2e-adminsuspendadmin-%")
	})

	target := register(t, env, targetEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	// Suspend for 24 hours
	suspendBody := `{"reason":"E2E test suspension","duration_hours":24}`
	suspendResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/users/"+target.UserID+"/suspend"), adminUser.AccessToken, suspendBody))
	defer func() { _ = suspendResp.Body.Close() }()
	suspendRaw, _ := io.ReadAll(suspendResp.Body)
	assert.Equal(t, http.StatusOK, suspendResp.StatusCode, "suspend failed: %s", string(suspendRaw))

	// Unsuspend
	unsuspendResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/users/"+target.UserID+"/unsuspend"), adminUser.AccessToken, ""))
	defer func() { _ = unsuspendResp.Body.Close() }()
	unsuspendRaw, _ := io.ReadAll(unsuspendResp.Body)
	assert.Equal(t, http.StatusOK, unsuspendResp.StatusCode, "unsuspend failed: %s", string(unsuspendRaw))
}

func TestE2E_Admin_ListAndDeletePost(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	userEmail := fmt.Sprintf("e2e-adminpostuser-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-adminpostadmin-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-adminpostuser-%")
		env.cleanupTestData(t, "e2e-adminpostadmin-%")
	})

	user := register(t, env, userEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	postID := createPost(t, env, user.AccessToken, "Post to be admin-deleted")

	// List all posts
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/posts"), adminUser.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "admin list posts failed: %s", string(listRaw))

	// Admin delete the post
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/posts/"+postID), adminUser.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "admin delete post failed: %s", string(delRaw))
}

func TestE2E_Admin_NonAdminGetsForbidden(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-admindenied-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-admindenied-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/stats"), tokens.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestE2E_Admin_UpdateUserRole(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	targetEmail := fmt.Sprintf("e2e-adminrole-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-adminroleadmin-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-adminrole-%")
		env.cleanupTestData(t, "e2e-adminroleadmin-%")
	})

	target := register(t, env, targetEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	body := `{"role":"moderator"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/users/"+target.UserID+"/role"), adminUser.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update role failed: %s", string(raw))
}

func TestE2E_Admin_CategoryCRUD(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-admincategory-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-admincategory-%") })

	adminUser := register(t, env, email, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	// Create category
	createBody := `{
		"name": "E2E Category",
		"icon": {"name":"home","library":"ionicons"},
		"color": "#FF5733",
		"status": "ACTIVE"
	}`
	createResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/categories"), adminUser.AccessToken, createBody))
	defer func() { _ = createResp.Body.Close() }()
	createRaw, _ := io.ReadAll(createResp.Body)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode, "create category failed: %s", string(createRaw))

	var createOut struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(createRaw, &createOut))
	categoryID := createOut.Data.ID
	require.NotEmpty(t, categoryID)

	// List categories
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/categories"), adminUser.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	// Update category
	updateBody := `{"name":"E2E Category Updated"}`
	updateResp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/categories/"+categoryID), adminUser.AccessToken, updateBody))
	defer func() { _ = updateResp.Body.Close() }()
	updateRaw, _ := io.ReadAll(updateResp.Body)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode, "update category failed: %s", string(updateRaw))

	// Delete category
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/categories/"+categoryID), adminUser.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "delete category failed: %s", string(delRaw))
}

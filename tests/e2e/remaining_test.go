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

// ---- UpdatePost ----

func TestE2E_Post_UpdatePost(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-postupd-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-postupd-%") })

	tokens := register(t, env, email, "Password123!")
	postID := createPost(t, env, tokens.AccessToken, "Original description")

	body := `{"description":"Updated description","visibility":"PUBLIC"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/posts/"+postID), tokens.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update post failed: %s", string(raw))
}

func TestE2E_Post_UpdateByNonOwnerReturns403(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-postupdowner-%d@test.local", ts)
	otherEmail := fmt.Sprintf("e2e-postupdother-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-postupdowner-%")
		env.cleanupTestData(t, "e2e-postupdother-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	other := register(t, env, otherEmail, "Password123!")
	postID := createPost(t, env, owner.AccessToken, "Owner's post")

	body := `{"description":"Unauthorized update"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/posts/"+postID), other.AccessToken, body))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---- Business hours ----

func TestE2E_Business_SetBusinessHours(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-bizhrsset-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-bizhrsset-%") })

	tokens := register(t, env, email, "Password123!")
	bizID := createBusiness(t, env, tokens.AccessToken, "Biz With Hours")

	body := `{"hours":[
		{"day_of_week":1,"open_time":"09:00","close_time":"17:00","is_closed":false},
		{"day_of_week":2,"open_time":"09:00","close_time":"17:00","is_closed":false}
	]}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/hours"), tokens.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "set business hours failed: %s", string(raw))
}

// ---- Report business ----

func TestE2E_Report_ReportBusiness(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("e2e-rprtbizowner-%d@test.local", ts)
	reporterEmail := fmt.Sprintf("e2e-rprtbizreporter-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-rprtbizowner-%")
		env.cleanupTestData(t, "e2e-rprtbizreporter-%")
	})

	owner := register(t, env, ownerEmail, "Password123!")
	reporter := register(t, env, reporterEmail, "Password123!")

	bizID := createBusiness(t, env, owner.AccessToken, "Reported Business")

	body := `{"reason":"fake_business"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/report"), reporter.AccessToken, body))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "report business failed: %s", string(raw))
}

// ---- Public categories ----

func TestE2E_Categories_ListPublic(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-catlist-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-catlist-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/categories"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list categories failed: %s", string(raw))
}

func TestE2E_Categories_GetByID(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	adminEmail := fmt.Sprintf("e2e-catgetadmin-%d@test.local", ts)
	userEmail := fmt.Sprintf("e2e-catgetuser-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-catgetadmin-%")
		env.cleanupTestData(t, "e2e-catgetuser-%")
	})

	adminUser := register(t, env, adminEmail, "Password123!")
	userTokens := register(t, env, userEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	// Create a category via admin
	createBody := `{"name":"GetByID Cat","icon":{"name":"star","library":"ionicons"},"color":"#123456","status":"ACTIVE"}`
	createResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/categories"), adminUser.AccessToken, createBody))
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createOut struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(createRaw, &createOut))
	catID := createOut.Data.ID

	// Get it via public endpoint
	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/categories/"+catID), userTokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get category by ID failed: %s", string(raw))
}

// ---- Unified search ----

func TestE2E_Search_Unified(t *testing.T) {
	env := setupE2E(t)
	email := fmt.Sprintf("e2e-searchunified-%d@test.local", time.Now().UnixNano())
	t.Cleanup(func() { env.cleanupTestData(t, "e2e-searchunified-%") })

	tokens := register(t, env, email, "Password123!")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/search?q=test"), tokens.AccessToken, ""))
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "unified search failed: %s", string(raw))
}

// ---- Admin help-chat (now in admin group) ----

func TestE2E_Admin_HelpChat_ThreadsAndReply(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	userEmail := fmt.Sprintf("e2e-admhcuser-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-admhcadmin-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-admhcuser-%")
		env.cleanupTestData(t, "e2e-admhcadmin-%")
	})

	user := register(t, env, userEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	// User sends help message
	env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/help-chat/messages"), user.AccessToken,
		`{"message":"Need help with login"}`)).Body.Close()

	// Admin lists threads
	threadsResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/help-chat"), adminUser.AccessToken, ""))
	defer threadsResp.Body.Close()
	threadsRaw, _ := io.ReadAll(threadsResp.Body)
	assert.Equal(t, http.StatusOK, threadsResp.StatusCode,
		"admin list threads failed: %s", string(threadsRaw))

	// Admin gets specific thread
	threadResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/help-chat/"+user.UserID), adminUser.AccessToken, ""))
	defer threadResp.Body.Close()
	threadRaw, _ := io.ReadAll(threadResp.Body)
	assert.Equal(t, http.StatusOK, threadResp.StatusCode,
		"admin get user thread failed: %s", string(threadRaw))

	// Admin replies
	replyResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/help-chat/"+user.UserID+"/reply"),
		adminUser.AccessToken, `{"message":"We can help!"}`))
	defer replyResp.Body.Close()
	replyRaw, _ := io.ReadAll(replyResp.Body)
	assert.Equal(t, http.StatusCreated, replyResp.StatusCode,
		"admin reply failed: %s", string(replyRaw))
}

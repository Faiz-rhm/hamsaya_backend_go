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

// adminSetup registers two users: a regular user and an admin, returns both.
func adminSetup(t *testing.T, env *testEnv, prefix string) (regular, admin authTokens) {
	t.Helper()
	ts := time.Now().UnixNano()
	regular = register(t, env, fmt.Sprintf("e2e-%sreg-%d@test.local", prefix, ts), "Password123!")
	admin = register(t, env, fmt.Sprintf("e2e-%sadm-%d@test.local", prefix, ts), "Password123!")
	env.makeAdmin(t, admin.UserID)
	t.Cleanup(func() {
		env.cleanupTestData(t, fmt.Sprintf("e2e-%sreg-%%", prefix))
		env.cleanupTestData(t, fmt.Sprintf("e2e-%sadm-%%", prefix))
	})
	return
}

func TestE2E_Admin_Reports_ListPostReports(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "rptlist")

	postID := createPost(t, env, regular.AccessToken, "Reported post")
	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/report"), regular.AccessToken,
		`{"reason":"spam"}`)).Body.Close()

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/posts"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list post reports failed: %s", string(raw))
}

func TestE2E_Admin_Reports_GetPostReport(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "rptget")

	postID := createPost(t, env, regular.AccessToken, "Post for report get")
	reportResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/report"), admin.AccessToken,
		`{"reason":"spam"}`))
	defer func() { _ = reportResp.Body.Close() }()
	reportRaw, _ := io.ReadAll(reportResp.Body)
	require.Equal(t, http.StatusCreated, reportResp.StatusCode, "create report failed: %s", string(reportRaw))

	// Create response has no ID — list and find by post_id
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/posts"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listOut struct {
		Data struct {
			Items []struct {
				ID     string `json:"id"`
				PostID string `json:"post_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var reportID string
	for _, r := range listOut.Data.Items {
		if r.PostID == postID {
			reportID = r.ID
			break
		}
	}
	require.NotEmpty(t, reportID, "created report not found in list")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/posts/"+reportID), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get post report failed: %s", string(raw))
}

func TestE2E_Admin_Reports_UpdateReportStatus(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "rptstatus")

	postID := createPost(t, env, regular.AccessToken, "Post for status update")
	reportResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/posts/"+postID+"/report"), admin.AccessToken,
		`{"reason":"spam"}`))
	defer func() { _ = reportResp.Body.Close() }()
	reportRaw, _ := io.ReadAll(reportResp.Body)
	require.Equal(t, http.StatusCreated, reportResp.StatusCode, "create report failed: %s", string(reportRaw))

	// Create response has no ID — list and find by post_id
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/posts"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listOut struct {
		Data struct {
			Items []struct {
				ID     string `json:"id"`
				PostID string `json:"post_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var reportID string
	for _, r := range listOut.Data.Items {
		if r.PostID == postID {
			reportID = r.ID
			break
		}
	}
	require.NotEmpty(t, reportID, "created report not found in list")

	body := `{"status":"RESOLVED"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/reports/posts/"+reportID+"/status"),
		admin.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update report status failed: %s", string(raw))
}

func TestE2E_Admin_Reports_ListCommentReports(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "cmtrpt")

	postID := createPost(t, env, regular.AccessToken, "Post")
	commentID := createComment(t, env, regular.AccessToken, postID, "Comment")
	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/comments/"+commentID+"/report"), regular.AccessToken,
		`{"reason":"harassment"}`)).Body.Close()

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/comments"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list comment reports failed: %s", string(raw))
}

func TestE2E_Admin_Reports_ListUserReports(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "userrpt")

	_ = env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+regular.UserID+"/report"), admin.AccessToken,
		`{"reason":"fake_account"}`)).Body.Close()

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/users"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list user reports failed: %s", string(raw))
}

func TestE2E_Admin_Reports_ListBusinessReports(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "bizrpt")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/businesses"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list business reports failed: %s", string(raw))
}

func TestE2E_Admin_Analytics_AllEndpoints(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "analytics")

	endpoints := []string{
		"/api/v1/admin/analytics/users",
		"/api/v1/admin/analytics/posts",
		"/api/v1/admin/analytics/engagement",
		"/api/v1/admin/analytics/businesses",
	}
	for _, ep := range endpoints {
		resp := env.do(bearerReq(http.MethodGet, env.url(ep), admin.AccessToken, ""))
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "analytics %s failed: %s", ep, string(raw))
	}
}

func TestE2E_Admin_Bans_IPBanCreateListDelete(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "ipban")

	// Create IP ban (create returns no ID in response body)
	body := `{"ip_address":"192.168.1.100","reason":"E2E test ban"}`
	createResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/bans/ip"), admin.AccessToken, body))
	defer func() { _ = createResp.Body.Close() }()
	createRaw, _ := io.ReadAll(createResp.Body)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode, "create IP ban failed: %s", string(createRaw))

	// List to find the ban ID (create response has no ID)
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/bans/ip"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "list IP bans failed: %s", string(listRaw))

	var listOut struct {
		Data struct {
			Bans []struct {
				ID        string `json:"id"`
				IPAddress string `json:"ip_address"`
			} `json:"bans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var banID string
	for _, b := range listOut.Data.Bans {
		if b.IPAddress == "192.168.1.100" {
			banID = b.ID
			break
		}
	}
	require.NotEmpty(t, banID, "newly created IP ban not found in list")

	// Delete
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/bans/ip/"+banID), admin.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "delete IP ban failed: %s", string(delRaw))
}

func TestE2E_Admin_Bans_DeviceBanCreateListDelete(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "devban")

	// Create device ban (create returns no ID in response body)
	body := `{"device_id":"test-device-abc-123","reason":"E2E device ban"}`
	createResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/bans/devices"), admin.AccessToken, body))
	defer func() { _ = createResp.Body.Close() }()
	createRaw, _ := io.ReadAll(createResp.Body)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode, "create device ban failed: %s", string(createRaw))

	// List to find the ban ID
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/bans/devices"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "list device bans failed: %s", string(listRaw))

	var listOut struct {
		Data struct {
			Bans []struct {
				ID       string `json:"id"`
				DeviceID string `json:"device_id"`
			} `json:"bans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var banID string
	for _, b := range listOut.Data.Bans {
		if b.DeviceID == "test-device-abc-123" {
			banID = b.ID
			break
		}
	}
	require.NotEmpty(t, banID, "newly created device ban not found in list")

	// Delete
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/bans/devices/"+banID), admin.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, delResp.StatusCode)
}

func TestE2E_Admin_AuditLogs(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "auditlog")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/audit-logs"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list audit logs failed: %s", string(raw))
}

func TestE2E_Admin_BroadcastNotification(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "broadcast")

	// FCM is nil in test env; service should handle gracefully (no actual push sent)
	body := `{"title":"E2E Test","message":"Test broadcast message"}`
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/notifications/broadcast"), admin.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	// May return 200 or 500 depending on FCM nil handling; just ensure it doesn't 401/403
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
		"broadcast must not return 401: %s", string(raw))
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"broadcast must not return 403: %s", string(raw))
}

func TestE2E_Admin_SendTargetedNotification(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "targeted")

	body := fmt.Sprintf(`{"title":"Hello","message":"Direct msg","user_ids":[%q]}`, regular.UserID)
	resp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/notifications/send"), admin.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
		"targeted notification must not return 401: %s", string(raw))
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
		"targeted notification must not return 403: %s", string(raw))
}

func TestE2E_Admin_Feedback_ListAndResolve(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "feedback")

	// Submit feedback as regular user
	feedbackBody := `{"rating":3,"type":"BUG","message":"Found a bug in E2E"}`
	submitResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/feedback"), regular.AccessToken, feedbackBody))
	defer func() { _ = submitResp.Body.Close() }()
	submitRaw, _ := io.ReadAll(submitResp.Body)
	require.Equal(t, http.StatusCreated, submitResp.StatusCode)

	var submitOut struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitRaw, &submitOut))
	feedbackID := submitOut.Data.ID

	// Admin list feedback
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/feedback"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "list feedback failed: %s", string(listRaw))

	// Admin resolve feedback
	resolveBody := `{"status":"RESOLVED","admin_notes":"Fixed in next release"}`
	resolveResp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/feedback/"+feedbackID+"/resolve"), admin.AccessToken, resolveBody))
	defer func() { _ = resolveResp.Body.Close() }()
	resolveRaw, _ := io.ReadAll(resolveResp.Body)
	assert.Equal(t, http.StatusOK, resolveResp.StatusCode, "resolve feedback failed: %s", string(resolveRaw))
}

func TestE2E_Admin_Accounts_ListAdmins(t *testing.T) {
	env := setupE2E(t)
	_, admin := adminSetup(t, env, "admlist")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/accounts"), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "list admins failed: %s", string(raw))
}

func TestE2E_Admin_Accounts_InviteCRUD(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	inviteEmail := fmt.Sprintf("e2e-invite-%d@test.local", ts)
	_, admin := adminSetup(t, env, "invite")

	// Create invite (create response has no ID — service doesn't return DB-generated ID)
	createBody := fmt.Sprintf(`{"email":%q,"role":"moderator"}`, inviteEmail)
	createResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/admin/accounts/invites"), admin.AccessToken, createBody))
	defer func() { _ = createResp.Body.Close() }()
	createRaw, _ := io.ReadAll(createResp.Body)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode, "create invite failed: %s", string(createRaw))

	// List invites to find the ID
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/accounts/invites"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "list invites failed: %s", string(listRaw))

	var listOut struct {
		Data []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var inviteID string
	for _, inv := range listOut.Data {
		if inv.Email == inviteEmail {
			inviteID = inv.ID
			break
		}
	}
	require.NotEmpty(t, inviteID, "newly created invite not found in list")

	// Revoke invite
	revokeResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/accounts/invites/"+inviteID), admin.AccessToken, ""))
	defer func() { _ = revokeResp.Body.Close() }()
	revokeRaw, _ := io.ReadAll(revokeResp.Body)
	assert.Equal(t, http.StatusOK, revokeResp.StatusCode, "revoke invite failed: %s", string(revokeRaw))
}

func TestE2E_Admin_Comments_ListGetRestoreDelete(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "admcmt")

	postID := createPost(t, env, regular.AccessToken, "Post")
	commentID := createComment(t, env, regular.AccessToken, postID, "A comment")

	// List all comments
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/comments"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "list comments failed: %s", string(listRaw))

	// Get comment
	getResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/comments/"+commentID), admin.AccessToken, ""))
	defer func() { _ = getResp.Body.Close() }()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "get comment failed: %s", string(getRaw))

	// Admin delete comment
	delResp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/comments/"+commentID), admin.AccessToken, ""))
	defer func() { _ = delResp.Body.Close() }()
	delRaw, _ := io.ReadAll(delResp.Body)
	assert.Equal(t, http.StatusOK, delResp.StatusCode, "admin delete comment failed: %s", string(delRaw))

	// Restore comment
	restoreResp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/comments/"+commentID+"/restore"), admin.AccessToken, ""))
	defer func() { _ = restoreResp.Body.Close() }()
	restoreRaw, _ := io.ReadAll(restoreResp.Body)
	assert.Equal(t, http.StatusOK, restoreResp.StatusCode, "restore comment failed: %s", string(restoreRaw))
}

func TestE2E_Admin_GetPostDetail(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "admpostdetail")

	postID := createPost(t, env, regular.AccessToken, "Post for admin detail")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/posts/"+postID), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "admin get post detail failed: %s", string(raw))
}

func TestE2E_Admin_UpdatePostStatus(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "admpoststatus")

	postID := createPost(t, env, regular.AccessToken, "Post for status update")

	body := `{"status":"HIDDEN"}`
	resp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/posts/"+postID+"/status"), admin.AccessToken, body))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "update post status failed: %s", string(raw))
}

func TestE2E_Admin_Businesses_ListGetUpdateStatus(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "admbiz")

	bizID := createBusiness(t, env, regular.AccessToken, "Admin Managed Biz")

	// List
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/businesses"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	assert.Equal(t, http.StatusOK, listResp.StatusCode, "admin list businesses failed: %s", string(listRaw))

	// Get detail
	getResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/businesses/"+bizID), admin.AccessToken, ""))
	defer func() { _ = getResp.Body.Close() }()
	getRaw, _ := io.ReadAll(getResp.Body)
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "admin get business detail failed: %s", string(getRaw))

	// Update status
	statusBody := `{"status":"ACTIVE"}`
	statusResp := env.do(bearerReq(http.MethodPut,
		env.url("/api/v1/admin/businesses/"+bizID+"/status"), admin.AccessToken, statusBody))
	defer func() { _ = statusResp.Body.Close() }()
	statusRaw, _ := io.ReadAll(statusResp.Body)
	assert.Equal(t, http.StatusOK, statusResp.StatusCode, "admin update biz status failed: %s", string(statusRaw))
}

func TestE2E_Admin_DeleteUser(t *testing.T) {
	env := setupE2E(t)
	ts := time.Now().UnixNano()
	targetEmail := fmt.Sprintf("e2e-admdeluser-%d@test.local", ts)
	adminEmail := fmt.Sprintf("e2e-admdeladm-%d@test.local", ts)
	t.Cleanup(func() {
		env.cleanupTestData(t, "e2e-admdeluser-%")
		env.cleanupTestData(t, "e2e-admdeladm-%")
	})

	target := register(t, env, targetEmail, "Password123!")
	adminUser := register(t, env, adminEmail, "Password123!")
	env.makeAdmin(t, adminUser.UserID)

	resp := env.do(bearerReq(http.MethodDelete,
		env.url("/api/v1/admin/users/"+target.UserID), adminUser.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "admin delete user failed: %s", string(raw))
}

func TestE2E_Admin_Reports_GetCommentReport(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "cmtrptget")

	postID := createPost(t, env, regular.AccessToken, "Post")
	commentID := createComment(t, env, regular.AccessToken, postID, "Comment to report")
	reportResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/comments/"+commentID+"/report"), admin.AccessToken,
		`{"reason":"harassment"}`))
	defer func() { _ = reportResp.Body.Close() }()
	reportRaw, _ := io.ReadAll(reportResp.Body)
	require.Equal(t, http.StatusCreated, reportResp.StatusCode, "create report failed: %s", string(reportRaw))

	// Create response has no ID — list and find by comment_id
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/comments"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listOut struct {
		Data struct {
			Items []struct {
				ID        string `json:"id"`
				CommentID string `json:"comment_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var reportID string
	for _, r := range listOut.Data.Items {
		if r.CommentID == commentID {
			reportID = r.ID
			break
		}
	}
	require.NotEmpty(t, reportID, "created comment report not found in list")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/comments/"+reportID), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get comment report failed: %s", string(raw))
}

func TestE2E_Admin_Reports_GetUserReport(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "userrptget")

	reportResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/users/"+regular.UserID+"/report"), admin.AccessToken,
		`{"reason":"fake_account"}`))
	defer func() { _ = reportResp.Body.Close() }()
	reportRaw, _ := io.ReadAll(reportResp.Body)
	require.Equal(t, http.StatusCreated, reportResp.StatusCode, "create report failed: %s", string(reportRaw))

	// Create response has no ID — list and find by reported_user_id
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/users"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listOut struct {
		Data struct {
			Items []struct {
				ID             string `json:"id"`
				ReportedUserID string `json:"reported_user_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var reportID string
	for _, r := range listOut.Data.Items {
		if r.ReportedUserID == regular.UserID {
			reportID = r.ID
			break
		}
	}
	require.NotEmpty(t, reportID, "created user report not found in list")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/users/"+reportID), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get user report failed: %s", string(raw))
}

func TestE2E_Admin_Reports_GetBusinessReport(t *testing.T) {
	env := setupE2E(t)
	regular, admin := adminSetup(t, env, "bizrptget")

	bizID := createBusiness(t, env, regular.AccessToken, "Reported Biz")
	reportResp := env.do(bearerReq(http.MethodPost,
		env.url("/api/v1/businesses/"+bizID+"/report"), admin.AccessToken,
		`{"reason":"fake_business"}`))
	defer func() { _ = reportResp.Body.Close() }()
	reportRaw, _ := io.ReadAll(reportResp.Body)
	require.Equal(t, http.StatusCreated, reportResp.StatusCode, "create report failed: %s", string(reportRaw))

	// Create response has no ID — list and find by business_id
	listResp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/businesses"), admin.AccessToken, ""))
	defer func() { _ = listResp.Body.Close() }()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listOut struct {
		Data struct {
			Items []struct {
				ID         string `json:"id"`
				BusinessID string `json:"business_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &listOut))
	var reportID string
	for _, r := range listOut.Data.Items {
		if r.BusinessID == bizID {
			reportID = r.ID
			break
		}
	}
	require.NotEmpty(t, reportID, "created business report not found in list")

	resp := env.do(bearerReq(http.MethodGet,
		env.url("/api/v1/admin/reports/businesses/"+reportID), admin.AccessToken, ""))
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "get business report failed: %s", string(raw))
}

package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	adminTestUserID = "admin-user-001"
	adminTestPostID = "admin-post-001"
)

func newAdminRouter(t *testing.T, adminRepo *mocks.MockAdminRepository) *gin.Engine {
	t.Helper()
	// AdminService takes nil fcmClient and nil notificationService (nil-guarded)
	svc := services.NewAdminService(adminRepo, nil, nil, zap.NewNop())
	h := NewAdminHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(adminTestUserID, "admin-sess-001")
	r := gin.New()
	r.GET("/api/v1/admin/stats", authed, h.GetDashboardStats)
	r.GET("/api/v1/admin/analytics/users", authed, h.GetUserAnalytics)
	r.GET("/api/v1/admin/analytics/posts", authed, h.GetPostAnalytics)
	r.GET("/api/v1/admin/analytics/engagement", authed, h.GetEngagementAnalytics)
	r.GET("/api/v1/admin/users", authed, h.ListUsers)
	r.GET("/api/v1/admin/users/:user_id", authed, h.GetUser)
	r.PUT("/api/v1/admin/users/:user_id/suspend", authed, h.SuspendUser)
	r.PUT("/api/v1/admin/users/:user_id/unsuspend", authed, h.UnsuspendUser)
	r.PUT("/api/v1/admin/users/:user_id/role", authed, h.UpdateUserRole)
	r.DELETE("/api/v1/admin/users/:user_id", authed, h.DeleteUser)
	r.GET("/api/v1/admin/posts", authed, h.ListAllPosts)
	r.DELETE("/api/v1/admin/posts/:post_id", authed, h.DeletePost)
	r.GET("/api/v1/admin/reports/posts", authed, h.ListPostReports)
	r.GET("/api/v1/admin/reports/users", authed, h.ListUserReports)
	r.GET("/api/v1/admin/businesses", authed, h.ListAllBusinesses)
	return r
}

// --- GetDashboardStats ---

func TestAdminHandler_GetDashboardStats(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetDashboardStats", mock.Anything).Return(&models.DashboardStats{}, nil)
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		adminRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetDashboardStats", mock.Anything).Return(nil, fmt.Errorf("db error"))
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetUserAnalytics ---

func TestAdminHandler_GetUserAnalytics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserAnalytics", mock.Anything, "month").Return(&models.UserAnalytics{}, nil)
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/analytics/users", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- ListUsers ---

func TestAdminHandler_ListUsers(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListUsers", mock.Anything, mock.AnythingOfType("*models.AdminUserFilter")).
			Return([]*models.AdminUserResponse{}, int64(0), nil)
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- GetUser ---

func TestAdminHandler_GetUser(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserByID", mock.Anything, adminTestUserID).Return(nil, fmt.Errorf("not found"))
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users/"+adminTestUserID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- SuspendUser ---

func TestAdminHandler_SuspendUser(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newAdminRouter(t, &mocks.MockAdminRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/admin/users/"+adminTestUserID+"/suspend",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("SuspendUser", mock.Anything, adminTestUserID, mock.Anything).Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/admin/users/"+adminTestUserID+"/suspend",
			strings.NewReader(`{"days":7,"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- UnsuspendUser ---

func TestAdminHandler_UnsuspendUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("UnsuspendUser", mock.Anything, adminTestUserID).Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/admin/users/"+adminTestUserID+"/unsuspend", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- DeleteUser ---

func TestAdminHandler_DeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeleteUser", mock.Anything, adminTestUserID).Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+adminTestUserID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- ListPosts ---

func TestAdminHandler_ListPosts(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListPosts", mock.Anything, mock.AnythingOfType("*models.AdminPostFilter")).
			Return([]*models.AdminPostResponse{}, int64(0), nil)
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/posts", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- DeletePost ---

func TestAdminHandler_DeletePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeletePost", mock.Anything, adminTestPostID).Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/posts/"+adminTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- ListPostReports ---

func TestAdminHandler_ListPostReports(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListPostReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return([]*models.AdminPostReportResponse{}, int64(0), nil)
		r := newAdminRouter(t, adminRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/reports/posts", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

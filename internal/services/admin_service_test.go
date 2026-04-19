package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// appErrMessage extracts the human-readable Message from an *utils.AppError.
// If the error is not an AppError it falls back to err.Error().
func appErrMessage(err error) string {
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return err.Error()
}

// newTestAdminService constructs an AdminService wired to the given mock repo.
// fcmClient and notificationService are intentionally nil for unit tests that
// do not exercise the broadcast path.
func newTestAdminService(adminRepo *mocks.MockAdminRepository) *AdminService {
	return NewAdminService(adminRepo, (*notification.FCMClient)(nil), nil, zap.NewNop())
}

// ---------------------------------------------------------------------------
// GetDashboardStats
// ---------------------------------------------------------------------------

func TestAdminService_GetDashboardStats(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockAdminRepository)
		expectStats   bool
		expectedError string
	}{
		{
			name: "success",
			setupMocks: func(r *mocks.MockAdminRepository) {
				stats := &models.DashboardStats{TotalUsers: 100, TotalPosts: 50}
				r.On("GetDashboardStats", mock.Anything).Return(stats, nil)
			},
			expectStats: true,
		},
		{
			name: "failure",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("GetDashboardStats", mock.Anything).Return(nil, errors.New("db error"))
			},
			expectedError: "Failed to get dashboard stats",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			stats, err := svc.GetDashboardStats(context.Background())

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
				assert.Nil(t, stats)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stats)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListUsers
// ---------------------------------------------------------------------------

func TestAdminService_ListUsers(t *testing.T) {
	tests := []struct {
		name          string
		filter        *models.AdminUserFilter
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:   "success",
			filter: &models.AdminUserFilter{Page: 1, Limit: 10},
			setupMocks: func(r *mocks.MockAdminRepository) {
				users := []*models.AdminUserResponse{
					{ID: "user-1", Email: "a@test.com"},
					{ID: "user-2", Email: "b@test.com"},
				}
				r.On("ListUsers", mock.Anything, mock.AnythingOfType("*models.AdminUserFilter")).
					Return(users, int64(2), nil)
			},
			expectedCount: 2,
		},
		{
			name:   "empty",
			filter: &models.AdminUserFilter{Page: 1, Limit: 10},
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("ListUsers", mock.Anything, mock.AnythingOfType("*models.AdminUserFilter")).
					Return([]*models.AdminUserResponse{}, int64(0), nil)
			},
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			resp, err := svc.ListUsers(context.Background(), tc.filter)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expectedCount, resp.TotalCount)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// GetUser
// ---------------------------------------------------------------------------

func TestAdminService_GetUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		setupMocks    func(*mocks.MockAdminRepository)
		expectUser    bool
		expectedError string
	}{
		{
			name:   "not found",
			userID: "user-999",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("GetUserByID", mock.Anything, "user-999").
					Return(nil, errors.New("not found"))
			},
			expectedError: "User not found",
		},
		{
			name:   "success",
			userID: "user-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				user := &models.AdminUserResponse{ID: "user-1", Email: "user@test.com"}
				r.On("GetUserByID", mock.Anything, "user-1").Return(user, nil)
			},
			expectUser: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			user, err := svc.GetUser(context.Background(), tc.userID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tc.userID, user.ID)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// SuspendUser
// ---------------------------------------------------------------------------

func TestAdminService_SuspendUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		days          int
		reason        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			userID:  "user-1",
			days:    7,
			reason:  "spam",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("SuspendUser", mock.Anything, "user-1", mock.AnythingOfType("time.Time")).
					Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:    "failure",
			userID:  "user-2",
			days:    3,
			reason:  "abuse",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("SuspendUser", mock.Anything, "user-2", mock.AnythingOfType("time.Time")).
					Return(errors.New("db error"))
			},
			expectedError: "Failed to suspend user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.SuspendUser(context.Background(), tc.userID, tc.days, tc.reason, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// UnsuspendUser
// ---------------------------------------------------------------------------

func TestAdminService_UnsuspendUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			userID:  "user-1",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UnsuspendUser", mock.Anything, "user-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.UnsuspendUser(context.Background(), tc.userID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateUserRole
// ---------------------------------------------------------------------------

func TestAdminService_UpdateUserRole(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		role          string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			userID:  "user-1",
			role:    "moderator",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdateUserRole", mock.Anything, "user-1", models.UserRole("moderator")).Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:    "repo failure",
			userID:  "user-2",
			role:    "admin",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdateUserRole", mock.Anything, "user-2", models.UserRole("admin")).
					Return(errors.New("db error"))
			},
			expectedError: "Failed to update user role",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.UpdateUserRole(context.Background(), tc.userID, tc.role, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteUser
// ---------------------------------------------------------------------------

func TestAdminService_DeleteUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			userID:  "user-1",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("DeleteUser", mock.Anything, "user-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.DeleteUser(context.Background(), tc.userID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListPosts
// ---------------------------------------------------------------------------

func TestAdminService_ListPosts(t *testing.T) {
	tests := []struct {
		name          string
		filter        *models.AdminPostFilter
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:   "success",
			filter: &models.AdminPostFilter{Page: 1, Limit: 10},
			setupMocks: func(r *mocks.MockAdminRepository) {
				posts := []*models.AdminPostResponse{
					{ID: "post-1"},
					{ID: "post-2"},
				}
				r.On("ListPosts", mock.Anything, mock.AnythingOfType("*models.AdminPostFilter")).
					Return(posts, int64(2), nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			resp, err := svc.ListPosts(context.Background(), tc.filter)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expectedCount, resp.TotalCount)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdatePostStatus
// ---------------------------------------------------------------------------

func TestAdminService_UpdatePostStatus(t *testing.T) {
	tests := []struct {
		name          string
		postID        string
		status        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			postID:  "post-1",
			status:  "active",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdatePostStatus", mock.Anything, "post-1", "active").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:    "repo failure",
			postID:  "post-2",
			status:  "hidden",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdatePostStatus", mock.Anything, "post-2", "hidden").
					Return(errors.New("db error"))
			},
			expectedError: "Failed to update post status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.UpdatePostStatus(context.Background(), tc.postID, tc.status, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// DeletePost
// ---------------------------------------------------------------------------

func TestAdminService_DeletePost(t *testing.T) {
	tests := []struct {
		name          string
		postID        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			postID:  "post-1",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("DeletePost", mock.Anything, "post-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.DeletePost(context.Background(), tc.postID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListBusinesses
// ---------------------------------------------------------------------------

func TestAdminService_ListBusinesses(t *testing.T) {
	tests := []struct {
		name          string
		filter        *models.AdminBusinessFilter
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:   "success",
			filter: &models.AdminBusinessFilter{Page: 1, Limit: 10},
			setupMocks: func(r *mocks.MockAdminRepository) {
				businesses := []*models.AdminBusinessResponse{
					{ID: "biz-1"},
					{ID: "biz-2"},
				}
				r.On("ListBusinesses", mock.Anything, mock.AnythingOfType("*models.AdminBusinessFilter")).
					Return(businesses, int64(2), nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			resp, err := svc.ListBusinesses(context.Background(), tc.filter)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expectedCount, resp.TotalCount)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateReportStatus
// ---------------------------------------------------------------------------

func TestAdminService_UpdateReportStatus(t *testing.T) {
	tests := []struct {
		name          string
		reportType    string
		reportID      string
		status        string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:          "invalid report type",
			reportType:    "unknown",
			reportID:      "rpt-1",
			status:        "RESOLVED",
			adminID:       "admin-1",
			setupMocks:    func(r *mocks.MockAdminRepository) {},
			expectedError: "Invalid report type",
		},
		{
			name:       "post type",
			reportType: "posts",
			reportID:   "rpt-1",
			status:     "RESOLVED",
			adminID:    "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdatePostReportStatus", mock.Anything, "rpt-1", "RESOLVED").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:       "comment type",
			reportType: "comments",
			reportID:   "rpt-2",
			status:     "RESOLVED",
			adminID:    "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdateCommentReportStatus", mock.Anything, "rpt-2", "RESOLVED").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:       "user type",
			reportType: "users",
			reportID:   "rpt-3",
			status:     "RESOLVED",
			adminID:    "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				// status == "RESOLVED" → resolved = true
				r.On("UpdateUserReportResolved", mock.Anything, "rpt-3", true).Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
		{
			name:       "business type",
			reportType: "businesses",
			reportID:   "rpt-4",
			status:     "RESOLVED",
			adminID:    "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("UpdateBusinessReportStatus", mock.Anything, "rpt-4", "RESOLVED").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.UpdateReportStatus(context.Background(), tc.reportType, tc.reportID, tc.status, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListAuditLogs
// ---------------------------------------------------------------------------

func TestAdminService_ListAuditLogs(t *testing.T) {
	tests := []struct {
		name          string
		filter        *models.AuditLogFilter
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:   "success",
			filter: &models.AuditLogFilter{Page: 1, Limit: 20},
			setupMocks: func(r *mocks.MockAdminRepository) {
				logs := []*models.AuditLog{
					{ID: "log-1", Action: "suspend_user"},
					{ID: "log-2", Action: "delete_post"},
				}
				r.On("ListAuditLogs", mock.Anything, mock.AnythingOfType("*models.AuditLogFilter")).
					Return(logs, int64(2), nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			items, total, err := svc.ListAuditLogs(context.Background(), tc.filter)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, items)
				assert.Equal(t, int64(0), total)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, total)
				assert.Len(t, items, int(tc.expectedCount))
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// CreateAdminInvite
// ---------------------------------------------------------------------------

func TestAdminService_CreateAdminInvite(t *testing.T) {
	tests := []struct {
		name          string
		req           *models.CreateAdminInviteRequest
		invitedBy     string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:      "success",
			req:       &models.CreateAdminInviteRequest{Email: "newadmin@test.com", Role: "moderator"},
			invitedBy: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				// token and expiresAt are generated internally; use Anything matchers.
				r.On("CreateAdminInvite", mock.Anything,
					"newadmin@test.com",
					mock.AnythingOfType("string"), // token hex
					"moderator",
					"admin-1",
					mock.AnythingOfType("time.Time"),
				).Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			invite, err := svc.CreateAdminInvite(context.Background(), tc.req, tc.invitedBy)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, invite)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, invite)
				assert.Equal(t, tc.req.Email, invite.Email)
				assert.Equal(t, tc.req.Role, invite.Role)
				assert.True(t, invite.ExpiresAt.After(time.Now()))
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListAdminInvites
// ---------------------------------------------------------------------------

func TestAdminService_ListAdminInvites(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int
		expectedError string
	}{
		{
			name: "success",
			setupMocks: func(r *mocks.MockAdminRepository) {
				invites := []*models.AdminInvite{
					{ID: "inv-1", Email: "a@test.com", Role: "moderator"},
					{ID: "inv-2", Email: "b@test.com", Role: "admin"},
				}
				r.On("ListAdminInvites", mock.Anything).Return(invites, nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			items, err := svc.ListAdminInvites(context.Background())

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, items)
			} else {
				assert.NoError(t, err)
				assert.Len(t, items, tc.expectedCount)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// RevokeAdminInvite
// ---------------------------------------------------------------------------

func TestAdminService_RevokeAdminInvite(t *testing.T) {
	tests := []struct {
		name          string
		inviteID      string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:     "success",
			inviteID: "inv-1",
			adminID:  "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("RevokeAdminInvite", mock.Anything, "inv-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.RevokeAdminInvite(context.Background(), tc.inviteID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListAdmins
// ---------------------------------------------------------------------------

func TestAdminService_ListAdmins(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int
		expectedError string
	}{
		{
			name: "success",
			setupMocks: func(r *mocks.MockAdminRepository) {
				admins := []*models.AdminActiveUser{
					{ID: "admin-1", Email: "admin@test.com", Role: "admin"},
					{ID: "mod-1", Email: "mod@test.com", Role: "moderator"},
				}
				r.On("ListAdmins", mock.Anything).Return(admins, nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			items, err := svc.ListAdmins(context.Background())

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, items)
			} else {
				assert.NoError(t, err)
				assert.Len(t, items, tc.expectedCount)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// CreateIPBan
// ---------------------------------------------------------------------------

func TestAdminService_CreateIPBan(t *testing.T) {
	tests := []struct {
		name          string
		req           *models.CreateIPBanRequest
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name: "success",
			req: &models.CreateIPBanRequest{
				IPAddress: "192.168.1.1",
				Reason:    testutil.StringPtr("abuse"),
				Days:      testutil.IntPtr(30),
			},
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("CreateIPBan", mock.Anything,
					"192.168.1.1",
					"admin-1",
					mock.Anything, // reason *string
					mock.Anything, // expiresAt *time.Time
				).Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.CreateIPBan(context.Background(), tc.req, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListIPBans
// ---------------------------------------------------------------------------

func TestAdminService_ListIPBans(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:  "success",
			page:  1,
			limit: 10,
			setupMocks: func(r *mocks.MockAdminRepository) {
				bans := []*models.IPBan{
					{ID: "ban-1", IPAddress: "10.0.0.1"},
					{ID: "ban-2", IPAddress: "10.0.0.2"},
				}
				r.On("ListIPBans", mock.Anything, 1, 10).Return(bans, int64(2), nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			items, total, err := svc.ListIPBans(context.Background(), tc.page, tc.limit)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, items)
				assert.Equal(t, int64(0), total)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, total)
				assert.Len(t, items, int(tc.expectedCount))
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteIPBan
// ---------------------------------------------------------------------------

func TestAdminService_DeleteIPBan(t *testing.T) {
	tests := []struct {
		name          string
		banID         string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			banID:   "ban-1",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("DeleteIPBan", mock.Anything, "ban-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.DeleteIPBan(context.Background(), tc.banID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// CreateDeviceBan
// ---------------------------------------------------------------------------

func TestAdminService_CreateDeviceBan(t *testing.T) {
	tests := []struct {
		name          string
		req           *models.CreateDeviceBanRequest
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name: "success",
			req: &models.CreateDeviceBanRequest{
				DeviceID: "device-abc",
				Reason:   testutil.StringPtr("fraud"),
				Days:     testutil.IntPtr(14),
			},
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("CreateDeviceBan", mock.Anything,
					"device-abc",
					"admin-1",
					mock.Anything, // reason *string
					mock.Anything, // expiresAt *time.Time
				).Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.CreateDeviceBan(context.Background(), tc.req, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// ListDeviceBans
// ---------------------------------------------------------------------------

func TestAdminService_ListDeviceBans(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		setupMocks    func(*mocks.MockAdminRepository)
		expectedCount int64
		expectedError string
	}{
		{
			name:  "success",
			page:  1,
			limit: 10,
			setupMocks: func(r *mocks.MockAdminRepository) {
				bans := []*models.DeviceBan{
					{ID: "dban-1", DeviceID: "device-001"},
					{ID: "dban-2", DeviceID: "device-002"},
				}
				r.On("ListDeviceBans", mock.Anything, 1, 10).Return(bans, int64(2), nil)
			},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			items, total, err := svc.ListDeviceBans(context.Background(), tc.page, tc.limit)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, items)
				assert.Equal(t, int64(0), total)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, total)
				assert.Len(t, items, int(tc.expectedCount))
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteDeviceBan
// ---------------------------------------------------------------------------

func TestAdminService_DeleteDeviceBan(t *testing.T) {
	tests := []struct {
		name          string
		banID         string
		adminID       string
		setupMocks    func(*mocks.MockAdminRepository)
		expectedError string
	}{
		{
			name:    "success",
			banID:   "dban-1",
			adminID: "admin-1",
			setupMocks: func(r *mocks.MockAdminRepository) {
				r.On("DeleteDeviceBan", mock.Anything, "dban-1").Return(nil)
				r.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).
					Return(nil)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adminRepo := &mocks.MockAdminRepository{}
			tc.setupMocks(adminRepo)

			svc := newTestAdminService(adminRepo)
			err := svc.DeleteDeviceBan(context.Background(), tc.banID, tc.adminID)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, appErrMessage(err), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
			adminRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

func TestAdminService_GetUserAnalytics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserAnalytics", mock.Anything, "30d").Return(&models.UserAnalytics{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetUserAnalytics(context.Background(), "30d")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		adminRepo.AssertExpectations(t)
	})
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserAnalytics", mock.Anything, "30d").Return(nil, errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetUserAnalytics(context.Background(), "30d")
		assert.Error(t, err)
	})
}

func TestAdminService_GetPostAnalytics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetPostAnalytics", mock.Anything, "7d").Return(&models.PostAnalytics{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetPostAnalytics(context.Background(), "7d")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetPostAnalytics", mock.Anything, "7d").Return(nil, errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetPostAnalytics(context.Background(), "7d")
		assert.Error(t, err)
	})
}

func TestAdminService_GetEngagementAnalytics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetEngagementAnalytics", mock.Anything, "30d").Return(&models.EngagementAnalytics{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetEngagementAnalytics(context.Background(), "30d")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetEngagementAnalytics", mock.Anything, "30d").Return(nil, errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetEngagementAnalytics(context.Background(), "30d")
		assert.Error(t, err)
	})
}

func TestAdminService_GetBusinessAnalytics(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetBusinessAnalytics", mock.Anything, "30d").Return(&models.BusinessAnalytics{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetBusinessAnalytics(context.Background(), "30d")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetBusinessAnalytics", mock.Anything, "30d").Return(nil, errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetBusinessAnalytics(context.Background(), "30d")
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// GetUserDetail
// ---------------------------------------------------------------------------

func TestAdminService_GetUserDetail(t *testing.T) {
	t.Run("user not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserByID", mock.Anything, "u-bad").Return(nil, errors.New("not found"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetUserDetail(context.Background(), "u-bad")
		assert.Error(t, err)
		assert.Contains(t, appErrMessage(err), "not found")
	})

	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		user := &models.AdminUserResponse{ID: "u-1"}
		adminRepo.On("GetUserByID", mock.Anything, "u-1").Return(user, nil)
		adminRepo.On("GetUserBio", mock.Anything, "u-1").Return((*string)(nil), nil)
		adminRepo.On("GetUserPosts", mock.Anything, "u-1", 10).Return([]*models.AdminPostResponse{}, nil)
		adminRepo.On("GetUserBusinesses", mock.Anything, "u-1").Return([]*models.AdminBusinessResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetUserDetail(context.Background(), "u-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		adminRepo.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// GetPostDetail
// ---------------------------------------------------------------------------

func TestAdminService_GetPostDetail(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetPostByID", mock.Anything, "p-bad").Return(nil, errors.New("not found"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetPostDetail(context.Background(), "p-bad")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		post := &models.AdminPostDetailResponse{}
		adminRepo.On("GetPostByID", mock.Anything, "p-1").Return(post, nil)
		adminRepo.On("GetPostComments", mock.Anything, "p-1").Return([]models.AdminPostCommentResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetPostDetail(context.Background(), "p-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		adminRepo.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Comment admin operations
// ---------------------------------------------------------------------------

func TestAdminService_GetComment(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetCommentByID", mock.Anything, "c-bad").Return(nil, errors.New("not found"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetComment(context.Background(), "c-bad")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		comment := &models.AdminCommentDetailResponse{}
		adminRepo.On("GetCommentByID", mock.Anything, "c-1").Return(comment, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetComment(context.Background(), "c-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_ListComments(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListComments", mock.Anything, mock.AnythingOfType("*models.AdminCommentFilter")).
			Return(nil, int64(0), errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.ListComments(context.Background(), &models.AdminCommentFilter{})
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListComments", mock.Anything, mock.AnythingOfType("*models.AdminCommentFilter")).
			Return([]*models.AdminCommentResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListComments(context.Background(), &models.AdminCommentFilter{Limit: 10})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_AdminDeleteComment(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeleteComment", mock.Anything, "c-1").Return(errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		err := svc.DeleteComment(context.Background(), "c-1", "admin-1")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeleteComment", mock.Anything, "c-1").Return(nil)
		adminRepo.On("ResolveCommentReportsByCommentID", mock.Anything, "c-1").Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).Return(nil)
		svc := newTestAdminService(adminRepo)
		err := svc.DeleteComment(context.Background(), "c-1", "admin-1")
		assert.NoError(t, err)
		adminRepo.AssertExpectations(t)
	})
}

func TestAdminService_RestoreComment(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("RestoreComment", mock.Anything, "c-1").Return(errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		err := svc.RestoreComment(context.Background(), "c-1", "admin-1")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("RestoreComment", mock.Anything, "c-1").Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).Return(nil)
		svc := newTestAdminService(adminRepo)
		err := svc.RestoreComment(context.Background(), "c-1", "admin-1")
		assert.NoError(t, err)
		adminRepo.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Business admin operations
// ---------------------------------------------------------------------------

func TestAdminService_GetBusinessDetail(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetBusinessByID", mock.Anything, "b-bad").Return(nil, errors.New("not found"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetBusinessDetail(context.Background(), "b-bad")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		biz := &models.AdminBusinessDetailResponse{}
		adminRepo.On("GetBusinessByID", mock.Anything, "b-1").Return(biz, nil)
		adminRepo.On("GetBusinessHours", mock.Anything, "b-1").Return([]models.AdminBusinessHour{}, nil)
		adminRepo.On("GetBusinessCategories", mock.Anything, "b-1").Return([]string{}, nil)
		adminRepo.On("GetBusinessGallery", mock.Anything, "b-1").Return([]models.AttachmentResponse{}, nil)
		adminRepo.On("GetBusinessPosts", mock.Anything, "b-1", 10).Return([]*models.AdminPostResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetBusinessDetail(context.Background(), "b-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		adminRepo.AssertExpectations(t)
	})
}

func TestAdminService_UpdateBusinessStatus(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("UpdateBusinessStatus", mock.Anything, "b-1", "suspended").Return(errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		err := svc.UpdateBusinessStatus(context.Background(), "b-1", "suspended", "admin-1")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("UpdateBusinessStatus", mock.Anything, "b-1", "active").Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).Return(nil)
		svc := newTestAdminService(adminRepo)
		err := svc.UpdateBusinessStatus(context.Background(), "b-1", "active", "admin-1")
		assert.NoError(t, err)
		adminRepo.AssertExpectations(t)
	})
}

func TestAdminService_AdminDeleteBusiness(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeleteBusiness", mock.Anything, "b-1").Return(errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		err := svc.DeleteBusiness(context.Background(), "b-1", "admin-1")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("DeleteBusiness", mock.Anything, "b-1").Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).Return(nil)
		svc := newTestAdminService(adminRepo)
		err := svc.DeleteBusiness(context.Background(), "b-1", "admin-1")
		assert.NoError(t, err)
		adminRepo.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Report admin operations
// ---------------------------------------------------------------------------

func TestAdminService_ListPostReports(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListPostReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return(nil, int64(0), errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.ListPostReports(context.Background(), &models.AdminReportFilter{})
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListPostReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return([]*models.AdminPostReportResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListPostReports(context.Background(), &models.AdminReportFilter{Limit: 10})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_GetPostReport(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetPostReportByID", mock.Anything, "r-bad").Return(nil, errors.New("not found"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.GetPostReport(context.Background(), "r-bad")
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetPostReportByID", mock.Anything, "r-1").Return(&models.AdminPostReportResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetPostReport(context.Background(), "r-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_ListCommentReports(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListCommentReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return([]*models.AdminCommentReportResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListCommentReports(context.Background(), &models.AdminReportFilter{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_GetCommentReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetCommentReportByID", mock.Anything, "r-1").Return(&models.AdminCommentReportResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetCommentReport(context.Background(), "r-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_ListUserReports(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListUserReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return([]*models.AdminUserReportResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListUserReports(context.Background(), &models.AdminReportFilter{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_GetUserReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetUserReportByID", mock.Anything, "r-1").Return(&models.AdminUserReportResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetUserReport(context.Background(), "r-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_ListBusinessReports(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListBusinessReports", mock.Anything, mock.AnythingOfType("*models.AdminReportFilter")).
			Return([]*models.AdminBusinessReportResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListBusinessReports(context.Background(), &models.AdminReportFilter{})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_GetBusinessReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("GetBusinessReportByID", mock.Anything, "r-1").Return(&models.AdminBusinessReportResponse{}, nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.GetBusinessReport(context.Background(), "r-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// ---------------------------------------------------------------------------
// BroadcastNotification
// ---------------------------------------------------------------------------

func TestAdminService_BroadcastNotification(t *testing.T) {
	t.Run("no notification service returns error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		svc := newTestAdminService(adminRepo)
		err := svc.BroadcastNotification(context.Background(), &models.BroadcastNotificationRequest{
			Title: "Hi", Message: "Msg",
		}, "admin-1")
		assert.Error(t, err)
	})

	t.Run("explicit user IDs", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		notifRepo := &mocks.MockNotificationRepository{}
		settingsRepo := &mocks.MockNotificationSettingsRepository{}
		notifSvc := NewNotificationService(notifRepo, settingsRepo, nil, nil, nil, nil, zap.NewNop())
		adminRepo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil).Maybe()
		notifRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Notification")).Return(nil)
		settingsRepo.On("GetByProfileID", mock.Anything, "u-1").Return([]*models.NotificationSetting{}, nil)

		svc := &AdminService{
			adminRepo:           adminRepo,
			notificationService: notifSvc,
			logger:              zap.NewNop(),
		}
		err := svc.BroadcastNotification(context.Background(), &models.BroadcastNotificationRequest{
			Title: "Hi", Message: "Msg", UserIDs: []string{"u-1"},
		}, "admin-1")
		assert.NoError(t, err)
		notifRepo.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// ListFeedback / ResolveFeedback
// ---------------------------------------------------------------------------

func TestAdminService_AdminListFeedback(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListFeedback", mock.Anything, mock.AnythingOfType("*models.AdminFeedbackFilter")).
			Return(nil, int64(0), errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		_, err := svc.ListFeedback(context.Background(), &models.AdminFeedbackFilter{})
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ListFeedback", mock.Anything, mock.AnythingOfType("*models.AdminFeedbackFilter")).
			Return([]*models.AdminFeedbackResponse{}, int64(0), nil)
		svc := newTestAdminService(adminRepo)
		result, err := svc.ListFeedback(context.Background(), &models.AdminFeedbackFilter{Limit: 10})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAdminService_AdminResolveFeedback(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ResolveFeedback", mock.Anything, "fb-1", "admin-1", "resolved", (*string)(nil)).
			Return(errors.New("db error"))
		svc := newTestAdminService(adminRepo)
		err := svc.ResolveFeedback(context.Background(), "fb-1", "admin-1", "resolved", nil)
		assert.Error(t, err)
	})
	t.Run("success", func(t *testing.T) {
		adminRepo := &mocks.MockAdminRepository{}
		adminRepo.On("ResolveFeedback", mock.Anything, "fb-1", "admin-1", "resolved", (*string)(nil)).Return(nil)
		adminRepo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*models.CreateAuditLogRequest")).Return(nil)
		svc := newTestAdminService(adminRepo)
		err := svc.ResolveFeedback(context.Background(), "fb-1", "admin-1", "resolved", nil)
		assert.NoError(t, err)
		adminRepo.AssertExpectations(t)
	})
}


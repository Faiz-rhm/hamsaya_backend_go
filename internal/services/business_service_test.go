package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// newTestBusinessService creates a BusinessService wired to the given mocks.
func newTestBusinessService(businessRepo *mocks.MockBusinessRepository, userRepo *mocks.MockUserRepository) *BusinessService {
	return NewBusinessService(businessRepo, userRepo, nil, zap.NewNop())
}

// ---------------------------------------------------------------------------
// TestBusinessService_CreateBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_CreateBusiness(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		req           *models.CreateBusinessRequest
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedError string
	}{
		{
			name:   "success",
			userID: "user-1",
			req: &models.CreateBusinessRequest{
				Name: "Acme Corp",
			},
			setupMocks: func(br *mocks.MockBusinessRepository) {
				// Create the business
				br.On("Create", mock.Anything, mock.AnythingOfType("*models.BusinessProfile")).Return(nil)
				// GetBusiness (called at end of CreateBusiness) calls GetByID then enrichBusiness helpers
				br.On("GetByID", mock.Anything, mock.AnythingOfType("string")).Return(
					testutil.CreateTestBusiness("biz-1", "user-1", "Acme Corp"), nil,
				)
				br.On("GetCategoriesByBusinessID", mock.Anything, mock.AnythingOfType("string")).Return([]*models.BusinessCategory{}, nil)
				br.On("GetHoursByBusinessID", mock.Anything, mock.AnythingOfType("string")).Return([]*models.BusinessHours{}, nil)
				br.On("IsFollowing", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(false, nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			resp, err := svc.CreateBusiness(context.Background(), tt.userID, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				if tt.expectedError != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_GetBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_GetBusiness(t *testing.T) {
	tests := []struct {
		name          string
		businessID    string
		viewerID      *string
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedError string
	}{
		{
			name:       "not found",
			businessID: "biz-999",
			viewerID:   nil,
			setupMocks: func(br *mocks.MockBusinessRepository) {
				br.On("GetByID", mock.Anything, "biz-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
		{
			name:       "success",
			businessID: "biz-1",
			viewerID:   strPtr("user-1"),
			setupMocks: func(br *mocks.MockBusinessRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				biz.Status = true
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
				br.On("GetCategoriesByBusinessID", mock.Anything, "biz-1").Return([]*models.BusinessCategory{}, nil)
				br.On("GetHoursByBusinessID", mock.Anything, "biz-1").Return([]*models.BusinessHours{}, nil)
				br.On("IsFollowing", mock.Anything, "biz-1", "user-1").Return(false, nil)
				// Non-owner triggers IncrementViews in a goroutine — allow it
				br.On("IncrementViews", mock.Anything, "biz-1").Return(nil).Maybe()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			resp, err := svc.GetBusiness(context.Background(), tt.businessID, tt.viewerID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_UpdateBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_UpdateBusiness(t *testing.T) {
	newName := "Updated Name"

	tests := []struct {
		name          string
		businessID    string
		userID        string
		req           *models.UpdateBusinessRequest
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedError string
	}{
		{
			name:       "not found",
			businessID: "biz-999",
			userID:     "user-1",
			req:        &models.UpdateBusinessRequest{Name: &newName},
			setupMocks: func(br *mocks.MockBusinessRepository) {
				br.On("GetByID", mock.Anything, "biz-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
		{
			name:       "not owner",
			businessID: "biz-1",
			userID:     "not-owner",
			req:        &models.UpdateBusinessRequest{Name: &newName},
			setupMocks: func(br *mocks.MockBusinessRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
			},
			expectError:   true,
			expectedError: "permission",
		},
		{
			name:       "success",
			businessID: "biz-1",
			userID:     "owner-1",
			req:        &models.UpdateBusinessRequest{Name: &newName},
			setupMocks: func(br *mocks.MockBusinessRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				biz.Status = true
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
				br.On("Update", mock.Anything, mock.AnythingOfType("*models.BusinessProfile")).Return(nil)
				// GetBusiness called at the end
				br.On("GetCategoriesByBusinessID", mock.Anything, "biz-1").Return([]*models.BusinessCategory{}, nil)
				br.On("GetHoursByBusinessID", mock.Anything, "biz-1").Return([]*models.BusinessHours{}, nil)
				br.On("IsFollowing", mock.Anything, "biz-1", "owner-1").Return(false, nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			resp, err := svc.UpdateBusiness(context.Background(), tt.businessID, tt.userID, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_DeleteBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_DeleteBusiness(t *testing.T) {
	tests := []struct {
		name          string
		businessID    string
		userID        string
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedError string
	}{
		{
			name:       "not found",
			businessID: "biz-999",
			userID:     "user-1",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				br.On("GetByID", mock.Anything, "biz-999").Return(nil, errors.New("record not found"))
			},
			expectError:   true,
			expectedError: "not found",
		},
		{
			name:       "not owner",
			businessID: "biz-1",
			userID:     "not-owner",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
			},
			expectError:   true,
			expectedError: "permission",
		},
		{
			name:       "success",
			businessID: "biz-1",
			userID:     "owner-1",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
				br.On("Delete", mock.Anything, "biz-1").Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			err := svc.DeleteBusiness(context.Background(), tt.businessID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_FollowBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_FollowBusiness(t *testing.T) {
	tests := []struct {
		name          string
		businessID    string
		userID        string
		setupMocks    func(*mocks.MockBusinessRepository, *mocks.MockUserRepository)
		expectError   bool
		expectedError string
	}{
		{
			// Follower is the same as the owner so the notification goroutine is skipped,
			// which avoids a nil-pointer dereference on the nil notificationService.
			name:       "success",
			businessID: "biz-1",
			userID:     "owner-1",
			setupMocks: func(br *mocks.MockBusinessRepository, ur *mocks.MockUserRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
				br.On("Follow", mock.Anything, "biz-1", "owner-1").Return(nil)
			},
			expectError: false,
		},
		{
			// Follower is the owner here too so the notification goroutine is skipped.
			name:       "failure",
			businessID: "biz-1",
			userID:     "owner-1",
			setupMocks: func(br *mocks.MockBusinessRepository, ur *mocks.MockUserRepository) {
				biz := testutil.CreateTestBusiness("biz-1", "owner-1", "Test Biz")
				br.On("GetByID", mock.Anything, "biz-1").Return(biz, nil)
				br.On("Follow", mock.Anything, "biz-1", "owner-1").Return(errors.New("db error"))
			},
			expectError:   true,
			expectedError: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo, userRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			err := svc.FollowBusiness(context.Background(), tt.businessID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				assert.NoError(t, err)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_UnfollowBusiness
// ---------------------------------------------------------------------------

func TestBusinessService_UnfollowBusiness(t *testing.T) {
	tests := []struct {
		name        string
		businessID  string
		userID      string
		setupMocks  func(*mocks.MockBusinessRepository)
		expectError bool
	}{
		{
			name:       "success",
			businessID: "biz-1",
			userID:     "user-1",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				br.On("Unfollow", mock.Anything, "biz-1", "user-1").Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			err := svc.UnfollowBusiness(context.Background(), tt.businessID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_GetUserBusinesses
// ---------------------------------------------------------------------------

func TestBusinessService_GetUserBusinesses(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "success",
			userID: "user-1",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				businesses := []*models.BusinessProfile{
					testutil.CreateTestBusiness("biz-1", "user-1", "Biz One"),
					testutil.CreateTestBusiness("biz-2", "user-1", "Biz Two"),
				}
				businesses[0].Status = true
				businesses[1].Status = true
				br.On("GetByUserID", mock.Anything, "user-1", 20, 0).Return(businesses, nil)
				// enrichBusiness calls for each business
				br.On("GetCategoriesByBusinessID", mock.Anything, mock.AnythingOfType("string")).Return([]*models.BusinessCategory{}, nil)
				br.On("GetHoursByBusinessID", mock.Anything, mock.AnythingOfType("string")).Return([]*models.BusinessHours{}, nil)
				br.On("IsFollowing", mock.Anything, mock.AnythingOfType("string"), "user-1").Return(false, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:   "empty",
			userID: "user-2",
			setupMocks: func(br *mocks.MockBusinessRepository) {
				br.On("GetByUserID", mock.Anything, "user-2", 20, 0).Return([]*models.BusinessProfile{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			resp, err := svc.GetUserBusinesses(context.Background(), tt.userID, 20, 0)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp, tt.expectedCount)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestBusinessService_GetAllCategories
// ---------------------------------------------------------------------------

func TestBusinessService_GetAllCategories(t *testing.T) {
	tests := []struct {
		name          string
		search        *string
		setupMocks    func(*mocks.MockBusinessRepository)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "success",
			search: nil,
			setupMocks: func(br *mocks.MockBusinessRepository) {
				categories := []*models.BusinessCategory{
					{ID: "cat-1", Name: "Food"},
					{ID: "cat-2", Name: "Tech"},
				}
				br.On("GetAllCategories", mock.Anything, (*string)(nil)).Return(categories, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			businessRepo := new(mocks.MockBusinessRepository)
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(businessRepo)

			svc := newTestBusinessService(businessRepo, userRepo)
			categories, err := svc.GetAllCategories(context.Background(), tt.search)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, categories, tt.expectedCount)
			}

			businessRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

// strPtr is a local helper (avoids importing testutil for tiny usage).
func strPtr(s string) *string { return &s }

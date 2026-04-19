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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRelationshipsService_FollowUser(t *testing.T) {
	tests := []struct {
		name          string
		followerID    string
		followingID   string
		setupMocks    func(*mocks.MockRelationshipsRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:        "follow yourself",
			followerID:  "user-123",
			followingID: "user-123",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
			},
			expectedError: "Cannot follow yourself",
		},
		{
			name:        "user not found",
			followerID:  "user-123",
			followingID: "user-999",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByID", mock.Anything, "user-999").Return(nil, errors.New("user not found"))
			},
			expectedError: "not found",
		},
		{
			name:        "already following",
			followerID:  "user-123",
			followingID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-456", "target@example.com")
				userRepo.On("GetByID", mock.Anything, "user-456").Return(user, nil)
				relRepo.On("IsFollowing", mock.Anything, "user-123", "user-456").Return(true, nil)
			},
			expectedError: "",
		},
		{
			name:        "successful follow",
			followerID:  "user-123",
			followingID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-456", "target@example.com")
				userRepo.On("GetByID", mock.Anything, "user-456").Return(user, nil)
				relRepo.On("IsFollowing", mock.Anything, "user-123", "user-456").Return(false, nil)
				relRepo.On("FollowUser", mock.Anything, "user-123", "user-456").Return(nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			relRepo := new(mocks.MockRelationshipsRepository)
			userRepo := new(mocks.MockUserRepository)

			tt.setupMocks(relRepo, userRepo)

			service := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())

			// Act
			err := service.FollowUser(context.Background(), tt.followerID, tt.followingID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			relRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestRelationshipsService_UnfollowUser(t *testing.T) {
	tests := []struct {
		name          string
		followerID    string
		followingID   string
		setupMocks    func(*mocks.MockRelationshipsRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:        "unfollow yourself",
			followerID:  "user-123",
			followingID: "user-123",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
			},
			expectedError: "Cannot unfollow yourself",
		},
		{
			name:        "successful unfollow",
			followerID:  "user-123",
			followingID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				relRepo.On("UnfollowUser", mock.Anything, "user-123", "user-456").Return(nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			relRepo := new(mocks.MockRelationshipsRepository)
			userRepo := new(mocks.MockUserRepository)

			tt.setupMocks(relRepo, userRepo)

			service := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())

			// Act
			err := service.UnfollowUser(context.Background(), tt.followerID, tt.followingID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			relRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestRelationshipsService_BlockUser(t *testing.T) {
	tests := []struct {
		name          string
		blockerID     string
		blockedID     string
		setupMocks    func(*mocks.MockRelationshipsRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:      "block yourself",
			blockerID: "user-123",
			blockedID: "user-123",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
			},
			expectedError: "Cannot block yourself",
		},
		{
			name:      "user not found",
			blockerID: "user-123",
			blockedID: "user-999",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByID", mock.Anything, "user-999").Return(nil, errors.New("user not found"))
			},
			expectedError: "not found",
		},
		{
			name:      "successful block",
			blockerID: "user-123",
			blockedID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-456", "target@example.com")
				userRepo.On("GetByID", mock.Anything, "user-456").Return(user, nil)
				relRepo.On("BlockUser", mock.Anything, "user-123", "user-456").Return(nil)
				// UnfollowUser is called twice (ignoring errors), so allow any call
				relRepo.On("UnfollowUser", mock.Anything, "user-123", "user-456").Return(nil).Maybe()
				relRepo.On("UnfollowUser", mock.Anything, "user-456", "user-123").Return(nil).Maybe()
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			relRepo := new(mocks.MockRelationshipsRepository)
			userRepo := new(mocks.MockUserRepository)

			tt.setupMocks(relRepo, userRepo)

			service := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())

			// Act
			err := service.BlockUser(context.Background(), tt.blockerID, tt.blockedID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			relRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestRelationshipsService_UnblockUser(t *testing.T) {
	tests := []struct {
		name          string
		blockerID     string
		blockedID     string
		setupMocks    func(*mocks.MockRelationshipsRepository, *mocks.MockUserRepository)
		expectedError string
	}{
		{
			name:      "unblock yourself",
			blockerID: "user-123",
			blockedID: "user-123",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
			},
			expectedError: "Cannot unblock yourself",
		},
		{
			name:      "successful unblock",
			blockerID: "user-123",
			blockedID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository, userRepo *mocks.MockUserRepository) {
				relRepo.On("UnblockUser", mock.Anything, "user-123", "user-456").Return(nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			relRepo := new(mocks.MockRelationshipsRepository)
			userRepo := new(mocks.MockUserRepository)

			tt.setupMocks(relRepo, userRepo)

			service := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())

			// Act
			err := service.UnblockUser(context.Background(), tt.blockerID, tt.blockedID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
			}

			relRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestRelationshipsService_GetRelationshipStatus(t *testing.T) {
	tests := []struct {
		name          string
		viewerID      string
		targetUserID  string
		setupMocks    func(*mocks.MockRelationshipsRepository)
		expectedError string
	}{
		{
			name:         "success",
			viewerID:     "user-123",
			targetUserID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository) {
				status := &models.RelationshipStatus{
					IsFollowing:  true,
					IsFollowedBy: false,
					IsBlocked:    false,
					HasBlockedMe: false,
				}
				relRepo.On("GetRelationshipStatus", mock.Anything, "user-123", "user-456").Return(status, nil)
			},
			expectedError: "",
		},
		{
			name:         "error",
			viewerID:     "user-123",
			targetUserID: "user-456",
			setupMocks: func(relRepo *mocks.MockRelationshipsRepository) {
				relRepo.On("GetRelationshipStatus", mock.Anything, "user-123", "user-456").Return(nil, errors.New("database error"))
			},
			expectedError: "database error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			relRepo := new(mocks.MockRelationshipsRepository)
			userRepo := new(mocks.MockUserRepository)

			tt.setupMocks(relRepo)

			service := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())

			// Act
			result, err := service.GetRelationshipStatus(context.Background(), tt.viewerID, tt.targetUserID)

			// Assert
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Nil(t, result)
				errMsg := strings.ToLower(err.Error())
				expectedMsg := strings.ToLower(tt.expectedError)
				assert.Contains(t, errMsg, expectedMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}

			relRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestRelationshipsService_GetFollowers(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetFollowers", mock.Anything, "user-1", 10, 0).
			Return(nil, errors.New("db error"))

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		_, err := svc.GetFollowers(context.Background(), "user-1", nil, 10, 0)
		require.Error(t, err)
	})

	t.Run("success empty", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetFollowers", mock.Anything, "user-1", 10, 0).
			Return([]*models.UserFollow{}, nil)

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		result, err := svc.GetFollowers(context.Background(), "user-1", nil, 10, 0)
		require.NoError(t, err)
		_ = result
	})
}

func TestRelationshipsService_GetFollowing(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetFollowing", mock.Anything, "user-1", 10, 0).
			Return(nil, errors.New("db error"))

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		_, err := svc.GetFollowing(context.Background(), "user-1", nil, 10, 0)
		require.Error(t, err)
	})

	t.Run("success empty", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetFollowing", mock.Anything, "user-1", 10, 0).
			Return([]*models.UserFollow{}, nil)

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		result, err := svc.GetFollowing(context.Background(), "user-1", nil, 10, 0)
		require.NoError(t, err)
		_ = result
	})
}

func TestRelationshipsService_GetBlockedUsers(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetBlockedUsers", mock.Anything, "user-1", 10, 0).
			Return(nil, errors.New("db error"))

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		_, err := svc.GetBlockedUsers(context.Background(), "user-1", 10, 0)
		require.Error(t, err)
	})

	t.Run("success empty", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := new(mocks.MockUserRepository)
		relRepo.On("GetBlockedUsers", mock.Anything, "user-1", 10, 0).
			Return([]*models.UserBlock{}, nil)

		svc := NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
		result, err := svc.GetBlockedUsers(context.Background(), "user-1", 10, 0)
		require.NoError(t, err)
		_ = result
	})
}

package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestProfileService(
	userRepo *mocks.MockUserRepository,
	postRepo *mocks.MockPostRepository,
	relRepo *mocks.MockRelationshipsRepository,
) *ProfileService {
	return NewProfileService(userRepo, postRepo, relRepo, zap.NewNop())
}

func TestProfileService_GetProfile(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository, *mocks.MockPostRepository, *mocks.MockRelationshipsRepository)
		userID        string
		viewerID      *string
		expectedError string
		check         func(*testing.T, *models.FullProfileResponse)
	}{
		{
			name: "user not found and not deleted",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, _ *mocks.MockRelationshipsRepository) {
				userRepo.On("GetByID", mock.Anything, "user-1").Return(nil, errors.New("not found"))
				userRepo.On("GetByIDIncludingDeleted", mock.Anything, "user-1").Return(nil, errors.New("not found"))
			},
			userID:        "user-1",
			expectedError: "not found",
		},
		{
			name: "deactivated user returns minimal profile",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, _ *mocks.MockRelationshipsRepository) {
				deletedAt := time.Now()
				deleted := testutil.CreateTestUser("user-del", "del@example.com")
				deleted.DeletedAt = &deletedAt
				userRepo.On("GetByID", mock.Anything, "user-del").Return(nil, errors.New("not found"))
				userRepo.On("GetByIDIncludingDeleted", mock.Anything, "user-del").Return(deleted, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-del").Return(3, nil)
			},
			userID: "user-del",
			check: func(t *testing.T, resp *models.FullProfileResponse) {
				assert.NotNil(t, resp)
			},
		},
		{
			name: "profile not found after user found",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *mocks.MockPostRepository, _ *mocks.MockRelationshipsRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(nil, errors.New("not found"))
			},
			userID:        "user-1",
			expectedError: "failed to get profile",
		},
		{
			name: "success without viewer",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, relRepo *mocks.MockRelationshipsRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				relRepo.On("GetFollowersCount", mock.Anything, "user-1").Return(10, nil)
				relRepo.On("GetFollowingCount", mock.Anything, "user-1").Return(5, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-1").Return(20, nil)
			},
			userID: "user-1",
			check: func(t *testing.T, resp *models.FullProfileResponse) {
				require.NotNil(t, resp)
				assert.Equal(t, 10, resp.FollowersCount)
				assert.Equal(t, 5, resp.FollowingCount)
				assert.Equal(t, 20, resp.PostsCount)
			},
		},
		{
			name: "success with viewer gets relationship status",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, relRepo *mocks.MockRelationshipsRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				relRepo.On("GetFollowersCount", mock.Anything, "user-1").Return(0, nil)
				relRepo.On("GetFollowingCount", mock.Anything, "user-1").Return(0, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-1").Return(0, nil)
				relRepo.On("GetRelationshipStatus", mock.Anything, "viewer-1", "user-1").
					Return(&models.RelationshipStatus{IsBlocked: false, HasBlockedMe: true}, nil)
			},
			userID:   "user-1",
			viewerID: testutil.StringPtr("viewer-1"),
			check: func(t *testing.T, resp *models.FullProfileResponse) {
				require.NotNil(t, resp)
				assert.False(t, resp.IsBlocked)
				assert.True(t, resp.HasBlockedMe)
			},
		},
		{
			name: "viewer same as user skips relationship check",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, relRepo *mocks.MockRelationshipsRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				relRepo.On("GetFollowersCount", mock.Anything, "user-1").Return(0, nil)
				relRepo.On("GetFollowingCount", mock.Anything, "user-1").Return(0, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-1").Return(0, nil)
				// GetRelationshipStatus should NOT be called when viewer == target
			},
			userID:   "user-1",
			viewerID: testutil.StringPtr("user-1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			postRepo := new(mocks.MockPostRepository)
			relRepo := new(mocks.MockRelationshipsRepository)
			tt.setupMocks(userRepo, postRepo, relRepo)
			svc := newTestProfileService(userRepo, postRepo, relRepo)

			resp, err := svc.GetProfile(context.Background(), tt.userID, tt.viewerID)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, resp)
				}
			}

			userRepo.AssertExpectations(t)
			postRepo.AssertExpectations(t)
			relRepo.AssertExpectations(t)
		})
	}
}

func TestProfileService_UpdateProfile(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockUserRepository, *mocks.MockPostRepository, *mocks.MockRelationshipsRepository)
		request       *models.UpdateProfileRequest
		expectedError string
	}{
		{
			name: "profile not found",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *mocks.MockPostRepository, _ *mocks.MockRelationshipsRepository) {
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").
					Return(nil, errors.New("not found"))
			},
			request:       &models.UpdateProfileRequest{},
			expectedError: "failed to get profile",
		},
		{
			name: "update fails",
			setupMocks: func(userRepo *mocks.MockUserRepository, _ *mocks.MockPostRepository, _ *mocks.MockRelationshipsRepository) {
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).
					Return(errors.New("db error"))
			},
			request:       &models.UpdateProfileRequest{FirstName: testutil.StringPtr("New")},
			expectedError: "failed to update profile",
		},
		{
			name: "success with field updates",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, relRepo *mocks.MockRelationshipsRepository) {
				profile := testutil.CreateTestProfile("user-1", "Old", "Name")
				user := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil).Once()
				userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)
				// GetProfile call from within UpdateProfile
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				relRepo.On("GetFollowersCount", mock.Anything, "user-1").Return(0, nil)
				relRepo.On("GetFollowingCount", mock.Anything, "user-1").Return(0, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-1").Return(0, nil)
			},
			request: &models.UpdateProfileRequest{
				FirstName: testutil.StringPtr("New"),
				LastName:  testutil.StringPtr("Name"),
			},
		},
		{
			name: "location update via flat fields",
			setupMocks: func(userRepo *mocks.MockUserRepository, postRepo *mocks.MockPostRepository, relRepo *mocks.MockRelationshipsRepository) {
				profile := testutil.CreateTestProfile("user-1", "Test", "User")
				user := testutil.CreateTestUser("user-1", "test@example.com")
				lat := 34.5
				lon := 69.2
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil).Once()
				userRepo.On("UpdateProfile", mock.Anything, mock.MatchedBy(func(p *models.Profile) bool {
					return p.Location != nil && p.Location.Valid
				})).Return(nil)
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
				relRepo.On("GetFollowersCount", mock.Anything, "user-1").Return(0, nil)
				relRepo.On("GetFollowingCount", mock.Anything, "user-1").Return(0, nil)
				postRepo.On("CountPostsByUser", mock.Anything, "user-1").Return(0, nil)
				_ = lat
				_ = lon
			},
			request: &models.UpdateProfileRequest{
				Latitude:  func() *float64 { v := 34.5; return &v }(),
				Longitude: func() *float64 { v := 69.2; return &v }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			postRepo := new(mocks.MockPostRepository)
			relRepo := new(mocks.MockRelationshipsRepository)
			tt.setupMocks(userRepo, postRepo, relRepo)
			svc := newTestProfileService(userRepo, postRepo, relRepo)

			_, err := svc.UpdateProfile(context.Background(), "user-1", tt.request)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestProfileService_UpdateAvatar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		profile := testutil.CreateTestProfile("user-1", "Test", "User")
		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
		userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.UpdateAvatar(context.Background(), "user-1", &models.Photo{
			URL: "https://example.com/avatar.jpg", Name: "avatar", MimeType: "image/jpeg",
		})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("profile not found", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").
			Return(nil, errors.New("not found"))

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.UpdateAvatar(context.Background(), "user-1", &models.Photo{URL: "x"})

		require.Error(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestProfileService_DeleteAvatar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		profile := testutil.CreateTestProfile("user-1", "Test", "User")
		photo := &models.Photo{URL: "https://example.com/old.jpg"}
		profile.Avatar = photo
		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
		userRepo.On("UpdateProfile", mock.Anything, mock.MatchedBy(func(p *models.Profile) bool {
			return p.Avatar == nil
		})).Return(nil)

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.DeleteAvatar(context.Background(), "user-1")

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestProfileService_UpdateCover(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		profile := testutil.CreateTestProfile("user-1", "Test", "User")
		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
		userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).Return(nil)

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.UpdateCover(context.Background(), "user-1", &models.Photo{
			URL: "https://example.com/cover.jpg", Name: "cover", MimeType: "image/jpeg",
		})

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestProfileService_DeleteCover(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		profile := testutil.CreateTestProfile("user-1", "Test", "User")
		photo := &models.Photo{URL: "https://example.com/cover.jpg"}
		profile.Cover = photo
		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").Return(profile, nil)
		userRepo.On("UpdateProfile", mock.Anything, mock.MatchedBy(func(p *models.Profile) bool {
			return p.Cover == nil
		})).Return(nil)

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.DeleteCover(context.Background(), "user-1")

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})
}

func TestProfileService_DeactivateAccount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		userRepo.On("SoftDelete", mock.Anything, "user-1").Return(nil)
		userRepo.On("RevokeAllUserSessions", mock.Anything, "user-1").Return(nil)

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.DeactivateAccount(context.Background(), "user-1")

		require.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("soft delete fails", func(t *testing.T) {
		userRepo := new(mocks.MockUserRepository)
		postRepo := new(mocks.MockPostRepository)
		relRepo := new(mocks.MockRelationshipsRepository)

		userRepo.On("SoftDelete", mock.Anything, "user-1").Return(errors.New("db error"))

		svc := newTestProfileService(userRepo, postRepo, relRepo)
		err := svc.DeactivateAccount(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "deactivate")
		userRepo.AssertExpectations(t)
	})
}

func TestProfileService_IsProfileComplete(t *testing.T) {
	svc := &ProfileService{}

	t.Run("nil location", func(t *testing.T) {
		profile := &models.Profile{Location: nil}
		assert.False(t, svc.isProfileComplete(profile))
	})

	t.Run("invalid location", func(t *testing.T) {
		profile := &models.Profile{
			Location: &pgtype.Point{Valid: false},
		}
		assert.False(t, svc.isProfileComplete(profile))
	})

	t.Run("valid location", func(t *testing.T) {
		profile := &models.Profile{
			Location: &pgtype.Point{P: pgtype.Vec2{X: 69.2, Y: 34.5}, Valid: true},
		}
		assert.True(t, svc.isProfileComplete(profile))
	})
}

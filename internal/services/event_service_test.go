package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestEventService(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) *EventService {
	return NewEventService(eventRepo, postRepo, userRepo, nil, zap.NewNop())
}

func newEventPost(postID string) *models.Post {
	pt := models.PostTypeEvent
	userID := "owner-1"
	return &models.Post{ID: postID, Type: pt, UserID: &userID}
}

func TestEventService_SetEventInterest(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockEventRepository, *mocks.MockPostRepository)
		state         models.EventInterestState
		expectedError string
	}{
		{
			name: "post not found",
			setupMocks: func(_ *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").
					Return(nil, errors.New("not found"))
			},
			state:         models.EventInterestInterested,
			expectedError: "post not found",
		},
		{
			name: "wrong post type",
			setupMocks: func(_ *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				post := &models.Post{ID: "post-1", Type: models.PostTypeFeed}
				postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)
			},
			state:         models.EventInterestInterested,
			expectedError: "event",
		},
		{
			name: "get existing interest error",
			setupMocks: func(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil)
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").
					Return(nil, errors.New("db error"))
			},
			state:         models.EventInterestInterested,
			expectedError: "failed to check",
		},
		{
			name: "same interest returns current status",
			setupMocks: func(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil).Twice()
				existing := &models.EventInterest{ID: "ei-1", UserID: "user-1", PostID: "post-1", EventState: models.EventInterestInterested}
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").Return(existing, nil)
				// GetEventInterestStatus also calls GetByID and GetUserInterest
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").Return(existing, nil)
			},
			state: models.EventInterestInterested,
		},
		{
			name: "new interest created",
			setupMocks: func(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil).Times(2)
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").Return(nil, nil).Twice()
				eventRepo.On("SetInterest", mock.Anything, mock.AnythingOfType("*models.EventInterest")).Return(nil)
			},
			state: models.EventInterestGoing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRepo := &mocks.MockEventRepository{}
			postRepo := &mocks.MockPostRepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(eventRepo, postRepo)
			svc := newTestEventService(eventRepo, postRepo, userRepo)

			resp, err := svc.SetEventInterest(context.Background(), "post-1", "user-1",
				&models.EventInterestRequest{EventState: tt.state})

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
			}

			eventRepo.AssertExpectations(t)
			postRepo.AssertExpectations(t)
		})
	}
}

func TestEventService_RemoveEventInterest(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockEventRepository, *mocks.MockPostRepository)
		expectedError string
	}{
		{
			name: "post not found",
			setupMocks: func(_ *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").
					Return(nil, errors.New("not found"))
			},
			expectedError: "post not found",
		},
		{
			name: "wrong post type",
			setupMocks: func(_ *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				post := &models.Post{ID: "post-1", Type: models.PostTypeSell}
				postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)
			},
			expectedError: "event",
		},
		{
			name: "no existing interest",
			setupMocks: func(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil)
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").Return(nil, nil)
			},
			expectedError: "has not expressed interest",
		},
		{
			name: "success",
			setupMocks: func(eventRepo *mocks.MockEventRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil)
				existing := &models.EventInterest{ID: "ei-1", UserID: "user-1", PostID: "post-1"}
				eventRepo.On("GetUserInterest", mock.Anything, "user-1", "post-1").Return(existing, nil)
				eventRepo.On("DeleteInterest", mock.Anything, "user-1", "post-1").Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRepo := &mocks.MockEventRepository{}
			postRepo := &mocks.MockPostRepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(eventRepo, postRepo)
			svc := newTestEventService(eventRepo, postRepo, userRepo)

			err := svc.RemoveEventInterest(context.Background(), "post-1", "user-1")

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			eventRepo.AssertExpectations(t)
			postRepo.AssertExpectations(t)
		})
	}
}

func TestEventService_GetEventInterestStatus(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		postRepo.On("GetByID", mock.Anything, "post-1").Return(nil, errors.New("not found"))

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		resp, err := svc.GetEventInterestStatus(context.Background(), "post-1", nil)

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success with viewer", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		post := newEventPost("post-1")
		post.InterestedCount = 5
		post.GoingCount = 3
		postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)
		userInterest := &models.EventInterest{EventState: models.EventInterestGoing}
		viewerID := "user-1"
		eventRepo.On("GetUserInterest", mock.Anything, viewerID, "post-1").Return(userInterest, nil)

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		resp, err := svc.GetEventInterestStatus(context.Background(), "post-1", &viewerID)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 5, resp.InterestedCount)
		assert.Equal(t, 3, resp.GoingCount)
		assert.Equal(t, models.EventInterestGoing, resp.UserEventState)
	})

	t.Run("success without viewer", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		post := newEventPost("post-1")
		postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		resp, err := svc.GetEventInterestStatus(context.Background(), "post-1", nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "post-1", resp.PostID)
	})
}

func TestEventService_GetInterestedUsers(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		postRepo.On("GetByID", mock.Anything, "post-1").Return(nil, errors.New("not found"))

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		result, err := svc.GetInterestedUsers(context.Background(), "post-1", 10, 0)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("success", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil)
		eventRepo.On("GetInterestedUsers", mock.Anything, "post-1", models.EventInterestInterested, 10, 0).
			Return([]*models.EventInterest{}, nil)

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		result, err := svc.GetInterestedUsers(context.Background(), "post-1", 10, 0)

		require.NoError(t, err)
		// empty input → enriched slice is nil (not initialized), which is fine
		_ = result
	})
}

func TestEventService_GetGoingUsers(t *testing.T) {
	t.Run("wrong post type", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		post := &models.Post{ID: "post-1", Type: models.PostTypeFeed}
		postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		result, err := svc.GetGoingUsers(context.Background(), "post-1", 10, 0)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("success", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		postRepo.On("GetByID", mock.Anything, "post-1").Return(newEventPost("post-1"), nil)
		eventRepo.On("GetInterestedUsers", mock.Anything, "post-1", models.EventInterestGoing, 5, 0).
			Return([]*models.EventInterest{}, nil)

		svc := newTestEventService(eventRepo, postRepo, userRepo)
		result, err := svc.GetGoingUsers(context.Background(), "post-1", 5, 0)

		require.NoError(t, err)
		_ = result
	})
}

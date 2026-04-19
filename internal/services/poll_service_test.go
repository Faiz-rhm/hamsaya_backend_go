package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestPollService(pollRepo *mocks.MockPollRepository, postRepo *mocks.MockPostRepository, userRepo *mocks.MockUserRepository) *PollService {
	return NewPollService(pollRepo, postRepo, userRepo, nil, zap.NewNop())
}

func newPullPost(postID string) *models.Post {
	pt := models.PostTypePull
	userID := "owner-1"
	return &models.Post{ID: postID, Type: pt, UserID: &userID}
}

func newTestPoll(pollID, postID string) *models.Poll {
	return &models.Poll{ID: pollID, PostID: postID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func TestPollService_CreatePoll(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockPollRepository, *mocks.MockPostRepository)
		expectedError string
	}{
		{
			name: "post not found",
			setupMocks: func(_ *mocks.MockPollRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(nil, errors.New("not found"))
			},
			expectedError: "post not found",
		},
		{
			name: "wrong post type",
			setupMocks: func(_ *mocks.MockPollRepository, postRepo *mocks.MockPostRepository) {
				post := &models.Post{ID: "post-1", Type: models.PostTypeFeed}
				postRepo.On("GetByID", mock.Anything, "post-1").Return(post, nil)
			},
			expectedError: "pull",
		},
		{
			name: "poll already exists",
			setupMocks: func(pollRepo *mocks.MockPollRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newPullPost("post-1"), nil)
				existing := newTestPoll("poll-1", "post-1")
				pollRepo.On("GetByPostID", mock.Anything, "post-1").Return(existing, nil)
			},
			expectedError: "already exists",
		},
		{
			name: "create poll fails",
			setupMocks: func(pollRepo *mocks.MockPollRepository, postRepo *mocks.MockPostRepository) {
				postRepo.On("GetByID", mock.Anything, "post-1").Return(newPullPost("post-1"), nil)
				pollRepo.On("GetByPostID", mock.Anything, "post-1").Return(nil, errors.New("not found"))
				pollRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Poll")).Return(errors.New("db error"))
			},
			expectedError: "failed to create poll",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pollRepo := &mocks.MockPollRepository{}
			postRepo := &mocks.MockPostRepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(pollRepo, postRepo)
			svc := newTestPollService(pollRepo, postRepo, userRepo)

			req := &models.CreatePollRequest{Options: []string{"Option A", "Option B"}}
			resp, err := svc.CreatePoll(context.Background(), "post-1", req)

			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			assert.Nil(t, resp)

			pollRepo.AssertExpectations(t)
			postRepo.AssertExpectations(t)
		})
	}
}

func TestPollService_GetPoll(t *testing.T) {
	t.Run("poll not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(nil, errors.New("not found"))

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.GetPoll(context.Background(), "poll-1", nil)

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success no viewer", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(poll, nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return([]*models.PollOption{}, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.GetPoll(context.Background(), "poll-1", nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "poll-1", resp.ID)
		assert.False(t, resp.HasVoted)
	})

	t.Run("success with viewer who has voted", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(poll, nil)
		opts := []*models.PollOption{
			{ID: "opt-1", PollID: "poll-1", Option: "A", VoteCount: 3},
			{ID: "opt-2", PollID: "poll-1", Option: "B", VoteCount: 1},
		}
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return(opts, nil)
		vote := &models.UserPoll{ID: "uv-1", UserID: "user-1", PollID: "poll-1", PollOptionID: "opt-1"}
		viewerID := "user-1"
		pollRepo.On("GetUserVote", mock.Anything, viewerID, "poll-1").Return(vote, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.GetPoll(context.Background(), "poll-1", &viewerID)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.HasVoted)
		assert.Equal(t, 4, resp.TotalVotes)
		assert.Equal(t, "opt-1", *resp.UserVote)
	})
}

func TestPollService_GetPollByPostID(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByPostID", mock.Anything, "post-1").Return(nil, errors.New("not found"))

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.GetPollByPostID(context.Background(), "post-1", nil)

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByPostID", mock.Anything, "post-1").Return(poll, nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return([]*models.PollOption{}, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.GetPollByPostID(context.Background(), "post-1", nil)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "post-1", resp.PostID)
	})
}

func TestPollService_VotePoll(t *testing.T) {
	t.Run("poll not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(nil, errors.New("not found"))

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "poll not found")
		assert.Nil(t, resp)
	})

	t.Run("option not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(newTestPoll("poll-1", "post-1"), nil)
		pollRepo.On("GetOptionByID", mock.Anything, "opt-bad").Return(nil, errors.New("not found"))

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-bad")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "option not found")
		assert.Nil(t, resp)
	})

	t.Run("option belongs to different poll", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(newTestPoll("poll-1", "post-1"), nil)
		wrongOpt := &models.PollOption{ID: "opt-1", PollID: "poll-other"}
		pollRepo.On("GetOptionByID", mock.Anything, "opt-1").Return(wrongOpt, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "does not belong")
		assert.Nil(t, resp)
	})

	t.Run("same option re-vote returns current state", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(poll, nil).Twice()
		pollRepo.On("GetOptionByID", mock.Anything, "opt-1").Return(&models.PollOption{ID: "opt-1", PollID: "poll-1"}, nil)
		existingVote := &models.UserPoll{UserID: "user-1", PollID: "poll-1", PollOptionID: "opt-1"}
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(existingVote, nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return([]*models.PollOption{}, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-1")

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("change vote to different option", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(poll, nil).Twice()
		pollRepo.On("GetOptionByID", mock.Anything, "opt-2").Return(&models.PollOption{ID: "opt-2", PollID: "poll-1"}, nil)
		existingVote := &models.UserPoll{UserID: "user-1", PollID: "poll-1", PollOptionID: "opt-1"}
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(existingVote, nil).Once()
		pollRepo.On("ChangeVote", mock.Anything, "user-1", "poll-1", "opt-2").Return(nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return([]*models.PollOption{}, nil)
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(nil, nil).Once()

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-2")

		require.NoError(t, err)
		assert.NotNil(t, resp)
		pollRepo.AssertExpectations(t)
	})

	t.Run("new vote", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		poll := newTestPoll("poll-1", "post-1")
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(poll, nil).Twice()
		pollRepo.On("GetOptionByID", mock.Anything, "opt-1").Return(&models.PollOption{ID: "opt-1", PollID: "poll-1"}, nil)
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(nil, nil).Once()
		pollRepo.On("VotePoll", mock.Anything, mock.AnythingOfType("*models.UserPoll")).Return(nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, "poll-1").Return([]*models.PollOption{}, nil)
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(nil, nil).Once()

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		resp, err := svc.VotePoll(context.Background(), "poll-1", "user-1", "opt-1")

		require.NoError(t, err)
		assert.NotNil(t, resp)
		pollRepo.AssertExpectations(t)
	})
}

func TestPollService_DeleteVote(t *testing.T) {
	t.Run("poll not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(nil, errors.New("not found"))

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		err := svc.DeleteVote(context.Background(), "poll-1", "user-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "poll not found")
	})

	t.Run("user has not voted", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(newTestPoll("poll-1", "post-1"), nil)
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(nil, nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		err := svc.DeleteVote(context.Background(), "poll-1", "user-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "not voted")
	})

	t.Run("success", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		pollRepo.On("GetByID", mock.Anything, "poll-1").Return(newTestPoll("poll-1", "post-1"), nil)
		vote := &models.UserPoll{UserID: "user-1", PollID: "poll-1", PollOptionID: "opt-1"}
		pollRepo.On("GetUserVote", mock.Anything, "user-1", "poll-1").Return(vote, nil)
		pollRepo.On("DeleteVote", mock.Anything, "user-1", "poll-1").Return(nil)

		svc := newTestPollService(pollRepo, postRepo, userRepo)
		err := svc.DeleteVote(context.Background(), "poll-1", "user-1")

		require.NoError(t, err)
		pollRepo.AssertExpectations(t)
	})
}

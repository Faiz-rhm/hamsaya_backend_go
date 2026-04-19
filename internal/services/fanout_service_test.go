package services

import (
	"context"
	"errors"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func newTestFanoutService(fanoutRepo *mocks.MockFanoutRepository) *FanoutService {
	return NewFanoutService(fanoutRepo, zap.NewNop())
}

func TestFanoutService_FanoutPost(t *testing.T) {
	t.Run("count followers error — no fanout", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		repo.On("CountFollowers", mock.Anything, "author-1").
			Return(0, errors.New("db error"))

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "author-1")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "GetFollowerIDs")
		repo.AssertNotCalled(t, "InsertFeedEntries")
	})

	t.Run("celebrity threshold exceeded — skip fanout", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		repo.On("CountFollowers", mock.Anything, "celebrity-1").
			Return(repositories.CelebrityThreshold+1, nil)

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "celebrity-1")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "GetFollowerIDs")
		repo.AssertNotCalled(t, "InsertFeedEntries")
	})

	t.Run("get follower IDs error — no insert", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		repo.On("CountFollowers", mock.Anything, "author-1").
			Return(100, nil)
		repo.On("GetFollowerIDs", mock.Anything, "author-1").
			Return(nil, errors.New("db error"))

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "author-1")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "InsertFeedEntries")
	})

	t.Run("success — inserts feed entries", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		followerIDs := []string{"f-1", "f-2", "f-3"}
		repo.On("CountFollowers", mock.Anything, "author-1").Return(3, nil)
		repo.On("GetFollowerIDs", mock.Anything, "author-1").Return(followerIDs, nil)
		repo.On("InsertFeedEntries", mock.Anything, "post-1", followerIDs).Return(nil)

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "author-1")

		repo.AssertExpectations(t)
	})

	t.Run("insert feed entries error — logged", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		followerIDs := []string{"f-1"}
		repo.On("CountFollowers", mock.Anything, "author-1").Return(1, nil)
		repo.On("GetFollowerIDs", mock.Anything, "author-1").Return(followerIDs, nil)
		repo.On("InsertFeedEntries", mock.Anything, "post-1", followerIDs).Return(errors.New("db error"))

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "author-1")

		repo.AssertExpectations(t)
	})

	t.Run("exactly at threshold — fanout proceeds", func(t *testing.T) {
		repo := &mocks.MockFanoutRepository{}
		followerIDs := []string{"f-1"}
		repo.On("CountFollowers", mock.Anything, "author-1").
			Return(repositories.CelebrityThreshold, nil)
		repo.On("GetFollowerIDs", mock.Anything, "author-1").Return(followerIDs, nil)
		repo.On("InsertFeedEntries", mock.Anything, "post-1", followerIDs).Return(nil)

		svc := newTestFanoutService(repo)
		svc.FanoutPost(context.Background(), "post-1", "author-1")

		repo.AssertExpectations(t)
	})
}

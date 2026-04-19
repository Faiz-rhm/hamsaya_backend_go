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

// newTestPostService builds a PostService wiring in only the repos the caller
// supplies; every other slot gets a fresh zero-value mock so the constructor
// does not panic.
func newTestPostService(
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
) *PostService {
	return NewPostService(
		postRepo,
		new(mocks.MockPollRepository),
		userRepo,
		new(mocks.MockBusinessRepository),
		new(mocks.MockRelationshipsRepository),
		new(mocks.MockCategoryRepository),
		new(mocks.MockEventRepository),
		nil, // notificationService
		nil, // fanoutService
		new(mocks.MockFanoutRepository),
		"hamsaya-uploads",
		zap.NewNop(),
	)
}

// ─── DeletePost ──────────────────────────────────────────────────────────────

func TestPostService_DeletePost(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(nil, errors.New("not found"))

		err := svc.DeletePost(context.Background(), "post-1", "user-1")

		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		postRepo.AssertExpectations(t)
	})

	t.Run("not owner", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		ownerID := "owner-user"
		post := testutil.CreateTestPost("post-1", ownerID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)

		err := svc.DeletePost(context.Background(), "post-1", "other-user")

		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "permission")
		postRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"
		post := testutil.CreateTestPost("post-1", userID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		postRepo.On("Delete", mock.Anything, "post-1").
			Return(nil)

		err := svc.DeletePost(context.Background(), "post-1", userID)

		assert.NoError(t, err)
		postRepo.AssertExpectations(t)
	})
}

// ─── LikePost ────────────────────────────────────────────────────────────────

func TestPostService_LikePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"
		post := testutil.CreateTestPost("post-1", userID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		postRepo.On("LikePost", mock.Anything, userID, "post-1").
			Return(nil)

		err := svc.LikePost(context.Background(), userID, "post-1")

		assert.NoError(t, err)
		postRepo.AssertExpectations(t)
	})

	t.Run("failure", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"
		post := testutil.CreateTestPost("post-1", userID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		postRepo.On("LikePost", mock.Anything, userID, "post-1").
			Return(errors.New("db error"))

		err := svc.LikePost(context.Background(), userID, "post-1")

		assert.Error(t, err)
		postRepo.AssertExpectations(t)
	})
}

// ─── UnlikePost ──────────────────────────────────────────────────────────────

func TestPostService_UnlikePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"

		postRepo.On("UnlikePost", mock.Anything, userID, "post-1").
			Return(nil)

		err := svc.UnlikePost(context.Background(), userID, "post-1")

		assert.NoError(t, err)
		postRepo.AssertExpectations(t)
	})
}

// ─── BookmarkPost ─────────────────────────────────────────────────────────────

func TestPostService_BookmarkPost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"
		post := testutil.CreateTestPost("post-1", userID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		postRepo.On("BookmarkPost", mock.Anything, userID, "post-1").
			Return(nil)

		err := svc.BookmarkPost(context.Background(), userID, "post-1")

		assert.NoError(t, err)
		postRepo.AssertExpectations(t)
	})
}

// ─── UnbookmarkPost ───────────────────────────────────────────────────────────

func TestPostService_UnbookmarkPost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		userID := "user-1"

		postRepo.On("UnbookmarkPost", mock.Anything, userID, "post-1").
			Return(nil)

		err := svc.UnbookmarkPost(context.Background(), userID, "post-1")

		assert.NoError(t, err)
		postRepo.AssertExpectations(t)
	})
}

// ─── GetPost ──────────────────────────────────────────────────────────────────

func TestPostService_GetPost(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(nil, errors.New("not found"))

		result, err := svc.GetPost(context.Background(), "post-1", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		postRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		svc := newTestPostService(postRepo, userRepo)

		viewerID := "viewer-1"
		ownerID := "owner-1"
		post := testutil.CreateTestPost("post-1", ownerID, models.PostTypeFeed)
		profile := testutil.CreateTestProfile("profile-1", "John", "Doe")

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		// enrichPost calls GetProfileByUserID for the author
		userRepo.On("GetProfileByUserID", mock.Anything, ownerID).
			Return(profile, nil)
		// GetAttachmentsByPostID is always called during enrichment
		postRepo.On("GetAttachmentsByPostID", mock.Anything, "post-1").
			Return(nil, errors.New("no attachments"))
		// GetEngagementStatus is called when viewerID is set
		postRepo.On("GetEngagementStatus", mock.Anything, viewerID, "post-1").
			Return(false, false, nil)

		result, err := svc.GetPost(context.Background(), "post-1", &viewerID)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "post-1", result.ID)
		postRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})
}

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

// newTestCommentService builds a CommentService with only the repos the caller
// supplies; notificationService is always nil (not exercised in unit tests).
func newTestCommentService(
	commentRepo *mocks.MockCommentRepository,
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
	businessRepo *mocks.MockBusinessRepository,
) *CommentService {
	return NewCommentService(
		commentRepo,
		postRepo,
		userRepo,
		businessRepo,
		nil, // notificationService
		zap.NewNop(),
	)
}

// buildComment constructs a minimal PostComment owned by ownerID on postID.
func buildComment(id, postID, ownerID string) *models.PostComment {
	return &models.PostComment{
		ID:     id,
		PostID: postID,
		UserID: ownerID,
		Text:   "test comment",
	}
}

// ─── CreateComment ────────────────────────────────────────────────────────────

func TestCommentService_CreateComment(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(nil, errors.New("not found"))

		req := &models.CreateCommentRequest{Text: "hello"}
		result, err := svc.CreateComment(context.Background(), "post-1", "user-1", req)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		postRepo.AssertExpectations(t)
		commentRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"
		post := testutil.CreateTestPost("post-1", userID, models.PostTypeFeed)
		profile := testutil.CreateTestProfile("profile-1", "Jane", "Doe")

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		// Create stores the new comment
		commentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PostComment")).
			Return(nil)
		// GetComment is called at the end of CreateComment with the newly generated UUID.
		// We return a stable comment — the test only checks that the result is non-nil.
		commentRepo.On("GetByID", mock.Anything, mock.AnythingOfType("string")).
			Return(buildComment("any-id", "post-1", userID), nil)

		// enrichComment calls GetProfileByUserID for the author
		userRepo.On("GetProfileByUserID", mock.Anything, userID).
			Return(profile, nil)
		// GetAttachmentsByCommentID
		commentRepo.On("GetAttachmentsByCommentID", mock.Anything, mock.AnythingOfType("string")).
			Return(nil, errors.New("no attachments"))
		// IsLikedByUser (viewerID == userID)
		commentRepo.On("IsLikedByUser", mock.Anything, userID, mock.AnythingOfType("string")).
			Return(false, nil)

		req := &models.CreateCommentRequest{Text: "hello"}
		result, err := svc.CreateComment(context.Background(), "post-1", userID, req)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		postRepo.AssertExpectations(t)
		commentRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})
}

// ─── DeleteComment ────────────────────────────────────────────────────────────

func TestCommentService_DeleteComment(t *testing.T) {
	t.Run("comment not found", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(nil, errors.New("not found"))

		err := svc.DeleteComment(context.Background(), "comment-1", "user-1")

		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		commentRepo.AssertExpectations(t)
	})

	t.Run("not owner", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		comment := buildComment("comment-1", "post-1", "owner-user")
		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)

		err := svc.DeleteComment(context.Background(), "comment-1", "other-user")

		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "permission")
		commentRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"
		comment := buildComment("comment-1", "post-1", userID)

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)
		commentRepo.On("Delete", mock.Anything, "comment-1").
			Return(nil)

		err := svc.DeleteComment(context.Background(), "comment-1", userID)

		assert.NoError(t, err)
		commentRepo.AssertExpectations(t)
	})
}

// ─── UpdateComment ────────────────────────────────────────────────────────────

func TestCommentService_UpdateComment(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(nil, errors.New("not found"))

		req := &models.UpdateCommentRequest{Text: "updated"}
		result, err := svc.UpdateComment(context.Background(), "comment-1", "user-1", req)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		commentRepo.AssertExpectations(t)
	})

	t.Run("not owner", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		comment := buildComment("comment-1", "post-1", "owner-user")
		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)

		req := &models.UpdateCommentRequest{Text: "updated"}
		result, err := svc.UpdateComment(context.Background(), "comment-1", "other-user", req)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, strings.ToLower(err.Error()), "permission")
		commentRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"
		comment := buildComment("comment-1", "post-1", userID)
		profile := testutil.CreateTestProfile("profile-1", "Jane", "Doe")

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)
		commentRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.PostComment")).
			Return(nil)
		// GetComment at the end of UpdateComment
		userRepo.On("GetProfileByUserID", mock.Anything, userID).
			Return(profile, nil)
		commentRepo.On("GetAttachmentsByCommentID", mock.Anything, "comment-1").
			Return(nil, errors.New("no attachments"))
		commentRepo.On("IsLikedByUser", mock.Anything, userID, "comment-1").
			Return(false, nil)

		req := &models.UpdateCommentRequest{Text: "updated"}
		result, err := svc.UpdateComment(context.Background(), "comment-1", userID, req)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		commentRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})
}

// ─── LikeComment ─────────────────────────────────────────────────────────────

func TestCommentService_LikeComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"
		comment := buildComment("comment-1", "post-1", userID)

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)
		commentRepo.On("LikeComment", mock.Anything, userID, "comment-1").
			Return(nil)

		err := svc.LikeComment(context.Background(), userID, "comment-1")

		assert.NoError(t, err)
		commentRepo.AssertExpectations(t)
	})
}

// ─── UnlikeComment ────────────────────────────────────────────────────────────

func TestCommentService_UnlikeComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"

		commentRepo.On("UnlikeComment", mock.Anything, userID, "comment-1").
			Return(nil)

		err := svc.UnlikeComment(context.Background(), userID, "comment-1")

		assert.NoError(t, err)
		commentRepo.AssertExpectations(t)
	})
}

// ─── GetComment ───────────────────────────────────────────────────────────────

func TestCommentService_GetComment(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(nil, errors.New("not found"))

		result, err := svc.GetComment(context.Background(), "comment-1", nil)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, strings.ToLower(err.Error()), "not found")
		commentRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		userID := "user-1"
		comment := buildComment("comment-1", "post-1", userID)
		profile := testutil.CreateTestProfile("profile-1", "John", "Doe")

		commentRepo.On("GetByID", mock.Anything, "comment-1").
			Return(comment, nil)
		userRepo.On("GetProfileByUserID", mock.Anything, userID).
			Return(profile, nil)
		commentRepo.On("GetAttachmentsByCommentID", mock.Anything, "comment-1").
			Return(nil, errors.New("no attachments"))
		// viewerID is nil so IsLikedByUser is NOT called

		result, err := svc.GetComment(context.Background(), "comment-1", nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "comment-1", result.ID)
		commentRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})
}

// ─── GetPostComments ──────────────────────────────────────────────────────────

func TestCommentService_GetPostComments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		ownerID := "owner-1"
		post := testutil.CreateTestPost("post-1", ownerID, models.PostTypeFeed)
		comment := buildComment("comment-1", "post-1", ownerID)
		profile := testutil.CreateTestProfile("profile-1", "John", "Doe")

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		commentRepo.On("GetByPostID", mock.Anything, "post-1", 10, 0).
			Return([]*models.PostComment{comment}, nil)
		// enrichComment for comment-1
		userRepo.On("GetProfileByUserID", mock.Anything, ownerID).
			Return(profile, nil)
		commentRepo.On("GetAttachmentsByCommentID", mock.Anything, "comment-1").
			Return(nil, errors.New("no attachments"))
		// No viewer → IsLikedByUser not called

		results, err := svc.GetPostComments(context.Background(), "post-1", 10, 0, nil)

		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, 1)
		postRepo.AssertExpectations(t)
		commentRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})

	t.Run("empty", func(t *testing.T) {
		commentRepo := new(mocks.MockCommentRepository)
		postRepo := new(mocks.MockPostRepository)
		userRepo := new(mocks.MockUserRepository)
		businessRepo := new(mocks.MockBusinessRepository)
		svc := newTestCommentService(commentRepo, postRepo, userRepo, businessRepo)

		ownerID := "owner-1"
		post := testutil.CreateTestPost("post-1", ownerID, models.PostTypeFeed)

		postRepo.On("GetByID", mock.Anything, "post-1").
			Return(post, nil)
		commentRepo.On("GetByPostID", mock.Anything, "post-1", 10, 0).
			Return([]*models.PostComment{}, nil)

		results, err := svc.GetPostComments(context.Background(), "post-1", 10, 0, nil)

		assert.NoError(t, err)
		// nil and empty slice are both acceptable empty results
		assert.Empty(t, results)
		postRepo.AssertExpectations(t)
		commentRepo.AssertExpectations(t)
	})
}

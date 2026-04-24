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
	commentTestUserID    = "comment-user-001"
	commentTestPostID    = "comment-post-001"
	commentTestCommentID = "comment-comment-001"
)

func newCommentRouter(
	t *testing.T,
	commentRepo *mocks.MockCommentRepository,
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewCommentService(
		commentRepo, postRepo, userRepo,
		&mocks.MockBusinessRepository{},
		nil, // notificationService nil-guarded
		zap.NewNop(),
	)
	h := NewCommentHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(commentTestUserID, "comment-sess-001")
	r := gin.New()
	r.POST("/api/v1/posts/:post_id/comments", authed, h.CreateComment)
	r.GET("/api/v1/posts/:post_id/comments", authed, h.GetPostComments)
	r.GET("/api/v1/comments/:comment_id", authed, h.GetComment)
	r.GET("/api/v1/comments/:comment_id/replies", authed, h.GetCommentReplies)
	r.PUT("/api/v1/comments/:comment_id", authed, h.UpdateComment)
	r.DELETE("/api/v1/comments/:comment_id", authed, h.DeleteComment)
	r.POST("/api/v1/comments/:comment_id/like", authed, h.LikeComment)
	r.DELETE("/api/v1/comments/:comment_id/like", authed, h.UnlikeComment)

	// Unauthed routes
	r.POST("/api/v1/noauth/posts/:post_id/comments", h.CreateComment)
	return r
}

// --- CreateComment ---

func TestCommentHandler_CreateComment(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newCommentRouter(t, &mocks.MockCommentRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/posts/"+commentTestPostID+"/comments",
			strings.NewReader(`{"text":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newCommentRouter(t, &mocks.MockCommentRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+commentTestPostID+"/comments",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing text field", func(t *testing.T) {
		r := newCommentRouter(t, &mocks.MockCommentRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+commentTestPostID+"/comments",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, commentTestPostID).Return(nil, fmt.Errorf("not found"))
		r := newCommentRouter(t, &mocks.MockCommentRepository{}, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+commentTestPostID+"/comments",
			strings.NewReader(`{"text":"hello world"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := &mocks.MockUserRepository{}
		post := testutil.CreateTestPost(commentTestPostID, "other-user", models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, commentTestPostID).Return(post, nil)
		commentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.PostComment")).Return(nil)
		commentRepo.On("GetByID", mock.Anything, mock.Anything).Return(&models.PostComment{
			ID:     "new-comment-id",
			UserID: commentTestUserID,
			PostID: commentTestPostID,
		}, nil)
		commentRepo.On("IsLikedByUser", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		commentRepo.On("GetAttachmentsByCommentID", mock.Anything, mock.Anything).Return([]*models.CommentAttachment{}, nil)
		userRepo.On("GetProfileByUserID", mock.Anything, mock.Anything).Return(&models.Profile{}, nil).Maybe()

		r := newCommentRouter(t, commentRepo, postRepo, userRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+commentTestPostID+"/comments",
			strings.NewReader(`{"text":"hello world"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- GetPostComments ---

func TestCommentHandler_GetPostComments(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(commentTestPostID, "other-user", models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, commentTestPostID).Return(post, nil)
		commentRepo.On("GetByPostID", mock.Anything, commentTestPostID, 20, 0).
			Return([]*models.PostComment{}, nil)
		r := newCommentRouter(t, commentRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/posts/"+commentTestPostID+"/comments", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
		commentRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(commentTestPostID, "other-user", models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, commentTestPostID).Return(post, nil)
		commentRepo.On("GetByPostID", mock.Anything, commentTestPostID, 20, 0).
			Return(nil, fmt.Errorf("db error"))
		r := newCommentRouter(t, commentRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/posts/"+commentTestPostID+"/comments", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetComment ---

func TestCommentHandler_GetComment(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).
			Return(nil, fmt.Errorf("not found"))
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/comments/"+commentTestCommentID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- DeleteComment ---

func TestCommentHandler_DeleteComment(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).
			Return(nil, fmt.Errorf("not found"))
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/comments/"+commentTestCommentID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("not owner", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		comment := &models.PostComment{ID: commentTestCommentID, UserID: "other-user"}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).Return(comment, nil)
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/comments/"+commentTestCommentID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		comment := &models.PostComment{ID: commentTestCommentID, UserID: commentTestUserID}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).Return(comment, nil)
		commentRepo.On("Delete", mock.Anything, commentTestCommentID).Return(nil)
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/comments/"+commentTestCommentID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		commentRepo.AssertExpectations(t)
	})
}

// --- LikeComment / UnlikeComment ---

func TestCommentHandler_LikeComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).Return(&models.PostComment{ID: commentTestCommentID, UserID: "other-user"}, nil)
		commentRepo.On("LikeComment", mock.Anything, commentTestUserID, commentTestCommentID).Return(nil)
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/comments/"+commentTestCommentID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("repo error", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		commentRepo.On("GetByID", mock.Anything, commentTestCommentID).Return(&models.PostComment{ID: commentTestCommentID, UserID: "other-user"}, nil)
		commentRepo.On("LikeComment", mock.Anything, commentTestUserID, commentTestCommentID).
			Return(fmt.Errorf("db error"))
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/comments/"+commentTestCommentID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestCommentHandler_UnlikeComment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		commentRepo := &mocks.MockCommentRepository{}
		commentRepo.On("UnlikeComment", mock.Anything, commentTestUserID, commentTestCommentID).Return(nil)
		r := newCommentRouter(t, commentRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/comments/"+commentTestCommentID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		commentRepo.AssertExpectations(t)
	})
}

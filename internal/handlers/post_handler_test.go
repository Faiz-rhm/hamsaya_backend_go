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
	postTestUserID = "post-test-user-001"
	postTestPostID = "post-test-post-001"
)

// buildPostService creates a PostService backed by mock repositories.
// notificationService is nil — PostService nil-guards all calls to it.
func buildPostService(
	postRepo *mocks.MockPostRepository,
	pollRepo *mocks.MockPollRepository,
	userRepo *mocks.MockUserRepository,
	businessRepo *mocks.MockBusinessRepository,
	relRepo *mocks.MockRelationshipsRepository,
	catRepo *mocks.MockCategoryRepository,
	eventRepo *mocks.MockEventRepository,
	fanoutRepo *mocks.MockFanoutRepository,
) *services.PostService {
	fanoutSvc := services.NewFanoutService(fanoutRepo, zap.NewNop())
	return services.NewPostService(
		postRepo, pollRepo, userRepo, businessRepo,
		relRepo, catRepo, eventRepo,
		nil, // notificationService — nil-guarded inside service
		fanoutSvc,
		fanoutRepo,
		"hamsaya-uploads",
		zap.NewNop(),
	)
}

// newPostRouter registers PostHandler routes the same way main.go does.
func newPostRouter(
	t *testing.T,
	postRepo *mocks.MockPostRepository,
	pollRepo *mocks.MockPollRepository,
	userRepo *mocks.MockUserRepository,
	businessRepo *mocks.MockBusinessRepository,
	relRepo *mocks.MockRelationshipsRepository,
	catRepo *mocks.MockCategoryRepository,
	eventRepo *mocks.MockEventRepository,
	fanoutRepo *mocks.MockFanoutRepository,
) *gin.Engine {
	t.Helper()
	svc := buildPostService(postRepo, pollRepo, userRepo, businessRepo, relRepo, catRepo, eventRepo, fanoutRepo)
	h := NewPostHandler(svc, nil, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(postTestUserID, "post-sess-001")
	r := gin.New()
	posts := r.Group("/api/v1/posts", authed)
	posts.GET("", h.GetFeed)
	posts.GET("/feed", h.GetPersonalizedFeed)
	posts.POST("", h.CreatePost)
	posts.GET("/:post_id", h.GetPost)
	posts.PUT("/:post_id", h.UpdatePost)
	posts.DELETE("/:post_id", h.DeletePost)
	posts.POST("/:post_id/like", h.LikePost)
	posts.DELETE("/:post_id/like", h.UnlikePost)
	posts.POST("/:post_id/bookmark", h.BookmarkPost)
	posts.DELETE("/:post_id/bookmark", h.UnbookmarkPost)
	posts.POST("/:post_id/share", h.SharePost)

	// unauthed route for testing missing user_id
	r.POST("/api/v1/posts-noauth", h.CreatePost)

	return r
}

func newMinimalPostRouter(t *testing.T, postRepo *mocks.MockPostRepository) *gin.Engine {
	t.Helper()
	return newPostRouter(t, postRepo,
		&mocks.MockPollRepository{},
		&mocks.MockUserRepository{},
		&mocks.MockBusinessRepository{},
		&mocks.MockRelationshipsRepository{},
		&mocks.MockCategoryRepository{},
		&mocks.MockEventRepository{},
		&mocks.MockFanoutRepository{},
	)
}

// --- CreatePost ---

func TestPostHandler_CreatePost(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newMinimalPostRouter(t, &mocks.MockPostRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts-noauth",
			strings.NewReader(`{"type":"FEED","description":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	tests := []struct {
		name       string
		body       string
		setupMocks func(*mocks.MockPostRepository)
		wantCode   int
	}{
		{
			name:       "invalid JSON",
			body:       `not-json`,
			setupMocks: func(r *mocks.MockPostRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "missing type field",
			body:       `{"description":"hello"}`,
			setupMocks: func(r *mocks.MockPostRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "invalid post type value",
			body:       `{"type":"INVALID","description":"hello"}`,
			setupMocks: func(r *mocks.MockPostRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name: "repository failure on create",
			body: `{"type":"FEED","description":"hello world"}`,
			setupMocks: func(r *mocks.MockPostRepository) {
				r.On("Create", mock.Anything, mock.AnythingOfType("*models.Post")).
					Return(fmt.Errorf("db error"))
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			postRepo := &mocks.MockPostRepository{}
			tc.setupMocks(postRepo)
			r := newMinimalPostRouter(t, postRepo)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts",
				strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
			postRepo.AssertExpectations(t)
		})
	}
}

// --- GetPost ---

func TestPostHandler_GetPost(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, postTestPostID).
			Return(nil, fmt.Errorf("not found"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/posts/"+postTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		postRepo.AssertExpectations(t)
	})
}

// --- DeletePost ---

func TestPostHandler_DeletePost(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, postTestPostID).
			Return(nil, fmt.Errorf("not found"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("not post owner", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		otherUser := "other-user-999"
		post := testutil.CreateTestPost(postTestPostID, otherUser, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("Delete", mock.Anything, postTestPostID).Return(nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := parseBody(t, w)
		assert.True(t, body["success"].(bool))
		postRepo.AssertExpectations(t)
	})

	t.Run("repo delete error", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("Delete", mock.Anything, postTestPostID).Return(fmt.Errorf("db error"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		postRepo.AssertExpectations(t)
	})
}

// --- LikePost ---

func TestPostHandler_LikePost(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, postTestPostID).
			Return(nil, fmt.Errorf("not found"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("success — own post so no notification goroutine", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		// Same owner as auth context → no notification goroutine started
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("LikePost", mock.Anything, postTestUserID, postTestPostID).Return(nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := parseBody(t, w)
		assert.True(t, body["success"].(bool))
		postRepo.AssertExpectations(t)
	})

	t.Run("like repo error", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("LikePost", mock.Anything, postTestUserID, postTestPostID).
			Return(fmt.Errorf("db error"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		postRepo.AssertExpectations(t)
	})
}

// --- UnlikePost ---

func TestPostHandler_UnlikePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("UnlikePost", mock.Anything, postTestUserID, postTestPostID).Return(nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("UnlikePost", mock.Anything, postTestUserID, postTestPostID).
			Return(fmt.Errorf("db error"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID+"/like", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		postRepo.AssertExpectations(t)
	})
}

// --- BookmarkPost ---

func TestPostHandler_BookmarkPost(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, postTestPostID).
			Return(nil, fmt.Errorf("not found"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/bookmark", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("BookmarkPost", mock.Anything, postTestUserID, postTestPostID).Return(nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/bookmark", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("bookmark repo error", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(postTestPostID, postTestUserID, models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, postTestPostID).Return(post, nil)
		postRepo.On("BookmarkPost", mock.Anything, postTestUserID, postTestPostID).
			Return(fmt.Errorf("db error"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+postTestPostID+"/bookmark", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		postRepo.AssertExpectations(t)
	})
}

// --- UnbookmarkPost ---

func TestPostHandler_UnbookmarkPost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("UnbookmarkPost", mock.Anything, postTestUserID, postTestPostID).Return(nil)
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID+"/bookmark", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		postRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("UnbookmarkPost", mock.Anything, postTestUserID, postTestPostID).
			Return(fmt.Errorf("db error"))
		r := newMinimalPostRouter(t, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/posts/"+postTestPostID+"/bookmark", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		postRepo.AssertExpectations(t)
	})
}

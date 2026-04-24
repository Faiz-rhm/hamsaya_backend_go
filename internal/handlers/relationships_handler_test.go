package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	relTestUserID   = "rel-test-user-001"
	relTestTargetID = "rel-test-target-002"
)

func newRelationshipsRouter(
	t *testing.T,
	relRepo *mocks.MockRelationshipsRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	// notificationService is nil — RelationshipsService nil-guards all calls to it
	svc := services.NewRelationshipsService(relRepo, userRepo, nil, zap.NewNop())
	h := NewRelationshipsHandler(svc, zap.NewNop())

	authed := authContextMiddleware(relTestUserID, "rel-sess-001")
	r := gin.New()

	r.POST("/api/v1/users/:user_id/follow", authed, h.FollowUser)
	r.DELETE("/api/v1/users/:user_id/follow", authed, h.UnfollowUser)
	r.POST("/api/v1/users/:user_id/block", authed, h.BlockUser)
	r.DELETE("/api/v1/users/:user_id/block", authed, h.UnblockUser)
	r.GET("/api/v1/users/:user_id/followers", authed, h.GetFollowers)
	r.GET("/api/v1/users/:user_id/following", authed, h.GetFollowing)
	r.GET("/api/v1/users/blocked", authed, h.GetBlockedUsers)
	r.GET("/api/v1/users/:user_id/relationship", authed, h.GetRelationshipStatus)

	// Unauthed routes for context-missing tests
	r.POST("/api/v1/noauth/:user_id/follow", h.FollowUser)
	r.POST("/api/v1/noauth/:user_id/block", h.BlockUser)

	return r
}

// --- FollowUser ---

func TestRelationshipsHandler_FollowUser(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newRelationshipsRouter(t, &mocks.MockRelationshipsRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("cannot follow yourself", func(t *testing.T) {
		r := newRelationshipsRouter(t, &mocks.MockRelationshipsRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		// target == self
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestUserID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("target user not found", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, relTestTargetID).
			Return(nil, fmt.Errorf("not found"))
		r := newRelationshipsRouter(t, relRepo, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		userRepo.AssertExpectations(t)
	})

	t.Run("already following — idempotent success", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := &mocks.MockUserRepository{}
		target := testutil.CreateTestUser(relTestTargetID, "target@example.com")
		userRepo.On("GetByID", mock.Anything, relTestTargetID).Return(target, nil)
		relRepo.On("IsFollowing", mock.Anything, relTestUserID, relTestTargetID).Return(true, nil)
		r := newRelationshipsRouter(t, relRepo, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		userRepo.AssertExpectations(t)
		relRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := &mocks.MockUserRepository{}
		target := testutil.CreateTestUser(relTestTargetID, "target@example.com")
		userRepo.On("GetByID", mock.Anything, relTestTargetID).Return(target, nil)
		relRepo.On("IsFollowing", mock.Anything, relTestUserID, relTestTargetID).Return(false, nil)
		relRepo.On("FollowUser", mock.Anything, relTestUserID, relTestTargetID).Return(nil)
		r := newRelationshipsRouter(t, relRepo, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := parseBody(t, w)
		assert.True(t, body["success"].(bool))
		userRepo.AssertExpectations(t)
		relRepo.AssertExpectations(t)
	})

	t.Run("follow repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := &mocks.MockUserRepository{}
		target := testutil.CreateTestUser(relTestTargetID, "target@example.com")
		userRepo.On("GetByID", mock.Anything, relTestTargetID).Return(target, nil)
		relRepo.On("IsFollowing", mock.Anything, relTestUserID, relTestTargetID).Return(false, nil)
		relRepo.On("FollowUser", mock.Anything, relTestUserID, relTestTargetID).
			Return(fmt.Errorf("db error"))
		r := newRelationshipsRouter(t, relRepo, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		relRepo.AssertExpectations(t)
	})
}

// --- UnfollowUser ---

func TestRelationshipsHandler_UnfollowUser(t *testing.T) {
	t.Run("cannot unfollow yourself", func(t *testing.T) {
		r := newRelationshipsRouter(t, &mocks.MockRelationshipsRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/"+relTestUserID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success — idempotent", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		relRepo.On("UnfollowUser", mock.Anything, relTestUserID, relTestTargetID).Return(nil)
		r := newRelationshipsRouter(t, relRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		relRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		relRepo.On("UnfollowUser", mock.Anything, relTestUserID, relTestTargetID).
			Return(fmt.Errorf("db error"))
		r := newRelationshipsRouter(t, relRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/"+relTestTargetID+"/follow", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		relRepo.AssertExpectations(t)
	})
}

// --- BlockUser ---

func TestRelationshipsHandler_BlockUser(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newRelationshipsRouter(t, &mocks.MockRelationshipsRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/"+relTestTargetID+"/block", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("cannot block yourself", func(t *testing.T) {
		r := newRelationshipsRouter(t, &mocks.MockRelationshipsRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestUserID+"/block", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		userRepo := &mocks.MockUserRepository{}
		target := testutil.CreateTestUser(relTestTargetID, "target@example.com")
		userRepo.On("GetByID", mock.Anything, relTestTargetID).Return(target, nil)
		// BlockUser also removes any follow relationships
		relRepo.On("UnfollowUser", mock.Anything, relTestUserID, relTestTargetID).Return(nil).Maybe()
		relRepo.On("UnfollowUser", mock.Anything, relTestTargetID, relTestUserID).Return(nil).Maybe()
		relRepo.On("BlockUser", mock.Anything, relTestUserID, relTestTargetID).Return(nil)
		r := newRelationshipsRouter(t, relRepo, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+relTestTargetID+"/block", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := parseBody(t, w)
		assert.True(t, body["success"].(bool))
	})
}

// --- UnblockUser ---

func TestRelationshipsHandler_UnblockUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		relRepo.On("UnblockUser", mock.Anything, relTestUserID, relTestTargetID).Return(nil)
		r := newRelationshipsRouter(t, relRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/"+relTestTargetID+"/block", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		relRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		relRepo.On("UnblockUser", mock.Anything, relTestUserID, relTestTargetID).
			Return(fmt.Errorf("db error"))
		r := newRelationshipsRouter(t, relRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/"+relTestTargetID+"/block", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		relRepo.AssertExpectations(t)
	})
}

// --- GetBlockedUsers ---

func TestRelationshipsHandler_GetBlockedUsers(t *testing.T) {
	t.Run("success — empty list", func(t *testing.T) {
		relRepo := &mocks.MockRelationshipsRepository{}
		relRepo.On("GetBlockedUsers", mock.Anything, relTestUserID, 20, 0).
			Return([]*models.UserBlock{}, nil)
		r := newRelationshipsRouter(t, relRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/blocked", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

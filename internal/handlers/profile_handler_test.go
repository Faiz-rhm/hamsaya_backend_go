package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	profileTestUserID   = "profile-test-user-001"
	profileTestTargetID = "profile-test-target-002"
)

func newProfileRouter(
	t *testing.T,
	userRepo *mocks.MockUserRepository,
	postRepo *mocks.MockPostRepository,
	relRepo *mocks.MockRelationshipsRepository,
) *gin.Engine {
	t.Helper()
	commentRepo := &mocks.MockCommentRepository{}
	svc := services.NewProfileService(userRepo, postRepo, commentRepo, relRepo, zap.NewNop())
	h := NewProfileHandler(svc, nil, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(profileTestUserID, "profile-sess-001")
	r := gin.New()

	r.GET("/api/v1/users/me", authed, h.GetMyProfile)
	r.PUT("/api/v1/users/me", authed, h.UpdateProfile)
	r.GET("/api/v1/users/:user_id", authed, h.GetUserProfile)

	// Unauthed routes for testing missing user_id
	r.GET("/api/v1/noauth/me", h.GetMyProfile)
	r.PUT("/api/v1/noauth/me", h.UpdateProfile)

	return r
}

// --- GetMyProfile ---

func TestProfileHandler_GetMyProfile(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newProfileRouter(t, &mocks.MockUserRepository{}, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/noauth/me", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("user not found — not active or deleted", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, profileTestUserID).
			Return(nil, fmt.Errorf("not found"))
		userRepo.On("GetByIDIncludingDeleted", mock.Anything, profileTestUserID).
			Return(nil, fmt.Errorf("not found"))
		r := newProfileRouter(t, userRepo, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		userRepo.AssertExpectations(t)
	})

	t.Run("profile fetch error", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		user := testutil.CreateTestUser(profileTestUserID, "me@example.com")
		userRepo.On("GetByID", mock.Anything, profileTestUserID).Return(user, nil)
		userRepo.On("GetProfileByUserID", mock.Anything, profileTestUserID).
			Return(nil, fmt.Errorf("db error"))
		r := newProfileRouter(t, userRepo, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})
}

// --- GetUserProfile ---

func TestProfileHandler_GetUserProfile(t *testing.T) {
	t.Run("target user not found", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, profileTestTargetID).
			Return(nil, fmt.Errorf("not found"))
		userRepo.On("GetByIDIncludingDeleted", mock.Anything, profileTestTargetID).
			Return(nil, fmt.Errorf("not found"))
		r := newProfileRouter(t, userRepo, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/"+profileTestTargetID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		userRepo.AssertExpectations(t)
	})

	t.Run("deactivated user returns minimal profile", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		postRepo := &mocks.MockPostRepository{}

		// GetByID fails, GetByIDIncludingDeleted returns deleted user
		deletedUser := testutil.CreateTestUser(profileTestTargetID, "deleted@example.com")
		now := deletedUser.CreatedAt
		deletedUser.DeletedAt = &now

		userRepo.On("GetByID", mock.Anything, profileTestTargetID).
			Return(nil, fmt.Errorf("not found"))
		userRepo.On("GetByIDIncludingDeleted", mock.Anything, profileTestTargetID).
			Return(deletedUser, nil)
		postRepo.On("CountPostsByUser", mock.Anything, profileTestTargetID).Return(0, nil)

		r := newProfileRouter(t, userRepo, postRepo, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/"+profileTestTargetID, nil)
		r.ServeHTTP(w, req)

		// Deactivated profile is returned, no 5xx
		assert.Less(t, w.Code, 500)
		userRepo.AssertExpectations(t)
		postRepo.AssertExpectations(t)
	})
}

// --- UpdateProfile ---

func TestProfileHandler_UpdateProfile(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newProfileRouter(t, &mocks.MockUserRepository{}, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/noauth/me",
			strings.NewReader(`{"first_name":"John"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newProfileRouter(t, &mocks.MockUserRepository{}, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/me",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("profile not found in DB", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetProfileByUserID", mock.Anything, profileTestUserID).
			Return(nil, fmt.Errorf("not found"))
		r := newProfileRouter(t, userRepo, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/me",
			strings.NewReader(`{"first_name":"John"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})

	t.Run("update repo error", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		profile := testutil.CreateTestProfile("prof-1", "John", "Doe")
		userRepo.On("GetProfileByUserID", mock.Anything, profileTestUserID).Return(profile, nil)
		userRepo.On("UpdateProfile", mock.Anything, mock.AnythingOfType("*models.Profile")).
			Return(fmt.Errorf("db error"))
		r := newProfileRouter(t, userRepo, &mocks.MockPostRepository{}, &mocks.MockRelationshipsRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/me",
			strings.NewReader(`{"first_name":"Jane"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})
}

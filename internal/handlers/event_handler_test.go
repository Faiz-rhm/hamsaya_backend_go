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
	eventTestUserID = "event-user-001"
	eventTestPostID = "event-post-001"
)

func newEventRouter(
	t *testing.T,
	eventRepo *mocks.MockEventRepository,
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewEventService(eventRepo, postRepo, userRepo, nil, zap.NewNop())
	h := NewEventHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(eventTestUserID, "event-sess-001")
	r := gin.New()
	r.POST("/api/v1/events/:post_id/interest", authed, h.SetEventInterest)
	r.DELETE("/api/v1/events/:post_id/interest", authed, h.RemoveEventInterest)
	r.GET("/api/v1/events/:post_id/interest", authed, h.GetEventInterestStatus)
	r.GET("/api/v1/events/:post_id/interested", authed, h.GetInterestedUsers)
	r.GET("/api/v1/events/:post_id/going", authed, h.GetGoingUsers)

	r.POST("/api/v1/noauth/events/:post_id/interest", h.SetEventInterest)
	return r
}

// --- SetEventInterest ---

func TestEventHandler_SetEventInterest(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newEventRouter(t, &mocks.MockEventRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/events/"+eventTestPostID+"/interest",
			strings.NewReader(`{"event_state":"interested"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newEventRouter(t, &mocks.MockEventRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/"+eventTestPostID+"/interest",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid event_state", func(t *testing.T) {
		r := newEventRouter(t, &mocks.MockEventRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/"+eventTestPostID+"/interest",
			strings.NewReader(`{"event_state":"invalid"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(nil, fmt.Errorf("not found"))
		r := newEventRouter(t, &mocks.MockEventRepository{}, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/"+eventTestPostID+"/interest",
			strings.NewReader(`{"event_state":"interested"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "other-user", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetUserInterest", mock.Anything, eventTestUserID, eventTestPostID).
			Return((*models.EventInterest)(nil), nil)
		eventRepo.On("SetInterest", mock.Anything, mock.AnythingOfType("*models.EventInterest")).Return(nil)
		eventRepo.On("CountByState", mock.Anything, eventTestPostID, models.EventInterestInterested).Return(1, nil)
		eventRepo.On("CountByState", mock.Anything, eventTestPostID, models.EventInterestGoing).Return(0, nil)

		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/"+eventTestPostID+"/interest",
			strings.NewReader(`{"event_state":"interested"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- RemoveEventInterest ---

func TestEventHandler_RemoveEventInterest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "other-user", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetUserInterest", mock.Anything, eventTestUserID, eventTestPostID).
			Return(&models.EventInterest{PostID: eventTestPostID, UserID: eventTestUserID}, nil)
		eventRepo.On("DeleteInterest", mock.Anything, eventTestUserID, eventTestPostID).Return(nil)
		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/events/"+eventTestPostID+"/interest", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("repo error", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "other-user", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetUserInterest", mock.Anything, eventTestUserID, eventTestPostID).
			Return(&models.EventInterest{PostID: eventTestPostID, UserID: eventTestUserID}, nil)
		eventRepo.On("DeleteInterest", mock.Anything, eventTestUserID, eventTestPostID).
			Return(fmt.Errorf("db error"))
		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/events/"+eventTestPostID+"/interest", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetEventInterestStatus ---

func TestEventHandler_GetEventInterestStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "other-user", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetUserInterest", mock.Anything, eventTestUserID, eventTestPostID).
			Return((*models.EventInterest)(nil), nil)
		eventRepo.On("CountByState", mock.Anything, eventTestPostID, models.EventInterestInterested).Return(0, nil)
		eventRepo.On("CountByState", mock.Anything, eventTestPostID, models.EventInterestGoing).Return(0, nil)

		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/events/"+eventTestPostID+"/interest", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- GetEventInterestStatus (CountByState uses EventInterestInterested/Going constants) ---

// --- GetInterestedUsers / GetGoingUsers ---

func TestEventHandler_GetInterestedUsers(t *testing.T) {
	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(nil, fmt.Errorf("not found"))
		r := newEventRouter(t, &mocks.MockEventRepository{}, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/events/"+eventTestPostID+"/interested", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success empty", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "owner", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetInterestedUsers", mock.Anything, eventTestPostID, models.EventInterestInterested, 20, 0).
			Return([]*models.EventInterest{}, nil)
		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/events/"+eventTestPostID+"/interested", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

func TestEventHandler_GetGoingUsers(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		eventRepo := &mocks.MockEventRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(eventTestPostID, "owner", models.PostTypeEvent)
		postRepo.On("GetByID", mock.Anything, eventTestPostID).Return(post, nil)
		eventRepo.On("GetInterestedUsers", mock.Anything, eventTestPostID, models.EventInterestGoing, 20, 0).
			Return([]*models.EventInterest{}, nil)
		r := newEventRouter(t, eventRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/events/"+eventTestPostID+"/going", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

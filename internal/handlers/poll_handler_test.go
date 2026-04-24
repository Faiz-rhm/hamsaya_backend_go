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
	pollTestUserID   = "poll-user-001"
	pollTestPostID   = "poll-post-001"
	pollTestPollID   = "poll-poll-001"
	pollTestOptionID = "11111111-1111-1111-1111-111111111111"
)

func newPollRouter(
	t *testing.T,
	pollRepo *mocks.MockPollRepository,
	postRepo *mocks.MockPostRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewPollService(pollRepo, postRepo, &mocks.MockUserRepository{}, nil, zap.NewNop())
	h := NewPollHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(pollTestUserID, "poll-sess-001")
	r := gin.New()
	r.POST("/api/v1/posts/:post_id/polls", authed, h.CreatePoll)
	r.GET("/api/v1/posts/:post_id/polls", authed, h.GetPostPoll)
	r.GET("/api/v1/polls/:poll_id", authed, h.GetPoll)
	r.POST("/api/v1/polls/:poll_id/vote", authed, h.VotePoll)
	r.DELETE("/api/v1/polls/:poll_id/vote", authed, h.DeleteVote)

	r.POST("/api/v1/noauth/polls/:poll_id/vote", h.VotePoll)
	return r
}

// --- CreatePoll ---

func TestPollHandler_CreatePoll(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newPollRouter(t, &mocks.MockPollRepository{}, &mocks.MockPostRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+pollTestPostID+"/polls",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("too few options", func(t *testing.T) {
		r := newPollRouter(t, &mocks.MockPollRepository{}, &mocks.MockPostRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+pollTestPostID+"/polls",
			strings.NewReader(`{"options":["only one"]}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, pollTestPostID).Return(nil, fmt.Errorf("not found"))
		r := newPollRouter(t, &mocks.MockPollRepository{}, postRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+pollTestPostID+"/polls",
			strings.NewReader(`{"options":["opt A","opt B"]}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(pollTestPostID, pollTestUserID, models.PostTypePull)
		postRepo.On("GetByID", mock.Anything, pollTestPostID).Return(post, nil)
		pollRepo.On("GetByPostID", mock.Anything, pollTestPostID).Return(nil, fmt.Errorf("not found"))
		pollRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Poll")).Return(nil)
		pollRepo.On("CreateOption", mock.Anything, mock.AnythingOfType("*models.PollOption")).Return(nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, mock.Anything).Return([]*models.PollOption{}, nil)
		pollRepo.On("GetByID", mock.Anything, mock.Anything).Return(&models.Poll{ID: "new-poll", PostID: pollTestPostID}, nil)

		r := newPollRouter(t, pollRepo, postRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+pollTestPostID+"/polls",
			strings.NewReader(`{"options":["opt A","opt B"]}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- GetPoll ---

func TestPollHandler_GetPoll(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		pollRepo.On("GetByID", mock.Anything, pollTestPollID).Return(nil, fmt.Errorf("not found"))
		r := newPollRouter(t, pollRepo, &mocks.MockPostRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/polls/"+pollTestPollID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- VotePoll ---

func TestPollHandler_VotePoll(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newPollRouter(t, &mocks.MockPollRepository{}, &mocks.MockPostRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/polls/"+pollTestPollID+"/vote",
			strings.NewReader(`{"poll_option_id":"`+pollTestOptionID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newPollRouter(t, &mocks.MockPollRepository{}, &mocks.MockPostRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/polls/"+pollTestPollID+"/vote",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("poll not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		pollRepo.On("GetByID", mock.Anything, pollTestPollID).Return(nil, fmt.Errorf("not found"))
		r := newPollRouter(t, pollRepo, &mocks.MockPostRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/polls/"+pollTestPollID+"/vote",
			strings.NewReader(`{"poll_option_id":"`+pollTestOptionID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- DeleteVote ---

func TestPollHandler_DeleteVote(t *testing.T) {
	t.Run("poll not found", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		pollRepo.On("GetByID", mock.Anything, pollTestPollID).Return(nil, fmt.Errorf("not found"))
		r := newPollRouter(t, pollRepo, &mocks.MockPostRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/polls/"+pollTestPollID+"/vote", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		pollRepo := &mocks.MockPollRepository{}
		poll := &models.Poll{ID: pollTestPollID, PostID: pollTestPostID}
		pollRepo.On("GetByID", mock.Anything, pollTestPollID).Return(poll, nil)
		pollRepo.On("GetUserVote", mock.Anything, pollTestUserID, pollTestPollID).
			Return(&models.UserPoll{UserID: pollTestUserID, PollID: pollTestPollID, PollOptionID: pollTestOptionID}, nil)
		pollRepo.On("DeleteVote", mock.Anything, pollTestUserID, pollTestPollID).Return(nil)
		pollRepo.On("UpdateOptionVoteCount", mock.Anything, pollTestOptionID, -1).Return(nil)
		pollRepo.On("GetOptionsByPollID", mock.Anything, pollTestPollID).Return([]*models.PollOption{}, nil)

		r := newPollRouter(t, pollRepo, &mocks.MockPostRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/polls/"+pollTestPollID+"/vote", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

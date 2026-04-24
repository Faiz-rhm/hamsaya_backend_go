package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const feedbackTestUserID = "feedback-user-001"

func newFeedbackRouter(t *testing.T, feedbackRepo *mocks.MockFeedbackRepository) *gin.Engine {
	t.Helper()
	svc := services.NewFeedbackService(feedbackRepo, testutil.CreateTestValidator())
	h := NewFeedbackHandler(svc)

	authed := authContextMiddleware(feedbackTestUserID, "feedback-sess-001")
	r := gin.New()
	r.POST("/api/v1/feedback", authed, h.SubmitFeedback)
	r.GET("/api/v1/feedback/status", authed, h.GetFeedbackStatus)

	// FeedbackHandler uses c.GetString("user_id") which returns "" if not set
	// so missing auth just produces empty userID (no 401 from handler itself)
	return r
}

// --- SubmitFeedback ---

func TestFeedbackHandler_SubmitFeedback(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newFeedbackRouter(t, &mocks.MockFeedbackRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/feedback",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		r := newFeedbackRouter(t, &mocks.MockFeedbackRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/feedback",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("repo error", func(t *testing.T) {
		feedbackRepo := &mocks.MockFeedbackRepository{}
		feedbackRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Feedback")).
			Return(fmt.Errorf("db error"))
		r := newFeedbackRouter(t, feedbackRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/feedback",
			strings.NewReader(`{"rating":4,"type":"GENERAL","message":"Great app!"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		feedbackRepo := &mocks.MockFeedbackRepository{}
		feedbackRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Feedback")).Return(nil)
		r := newFeedbackRouter(t, feedbackRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/feedback",
			strings.NewReader(`{"rating":4,"type":"GENERAL","message":"Great app!"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- GetFeedbackStatus ---

func TestFeedbackHandler_GetFeedbackStatus(t *testing.T) {
	t.Run("success — no prior feedback", func(t *testing.T) {
		feedbackRepo := &mocks.MockFeedbackRepository{}
		feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, feedbackTestUserID).
			Return(false, (*time.Time)(nil), nil)
		r := newFeedbackRouter(t, feedbackRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/feedback/status", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})

	t.Run("repo error", func(t *testing.T) {
		feedbackRepo := &mocks.MockFeedbackRepository{}
		feedbackRepo.On("GetUserFeedbackStatus", mock.Anything, feedbackTestUserID).
			Return(false, (*time.Time)(nil), fmt.Errorf("db error"))
		r := newFeedbackRouter(t, feedbackRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/feedback/status", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}


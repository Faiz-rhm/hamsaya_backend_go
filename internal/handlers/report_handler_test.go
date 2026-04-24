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
)

const (
	reportTestUserID     = "report-user-001"
	reportTestPostID     = "report-post-001"
	reportTestCommentID  = "report-comment-001"
	reportTestTargetUID  = "report-target-001"
	reportTestBusinessID = "report-biz-001"
)

func newReportRouter(
	t *testing.T,
	reportRepo *mocks.MockReportRepository,
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewReportService(
		reportRepo, postRepo, userRepo,
		testutil.CreateTestValidator(),
	)
	h := NewReportHandler(svc)

	authed := authContextMiddleware(reportTestUserID, "report-sess-001")
	r := gin.New()
	r.POST("/api/v1/posts/:post_id/report", authed, h.ReportPost)
	r.POST("/api/v1/comments/:comment_id/report", authed, h.ReportComment)
	r.POST("/api/v1/users/:user_id/report", authed, h.ReportUser)
	r.POST("/api/v1/businesses/:business_id/report", authed, h.ReportBusiness)
	return r
}

// --- ReportPost ---

func TestReportHandler_ReportPost(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newReportRouter(t, &mocks.MockReportRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+reportTestPostID+"/report",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("post not found", func(t *testing.T) {
		postRepo := &mocks.MockPostRepository{}
		postRepo.On("GetByID", mock.Anything, reportTestPostID).Return(nil, fmt.Errorf("not found"))
		r := newReportRouter(t, &mocks.MockReportRepository{}, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+reportTestPostID+"/report",
			strings.NewReader(`{"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		reportRepo := &mocks.MockReportRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(reportTestPostID, "other-user", models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, reportTestPostID).Return(post, nil)
		reportRepo.On("CreatePostReport", mock.Anything, mock.AnythingOfType("*models.PostReport")).Return(nil)
		r := newReportRouter(t, reportRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+reportTestPostID+"/report",
			strings.NewReader(`{"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		reportRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		reportRepo := &mocks.MockReportRepository{}
		postRepo := &mocks.MockPostRepository{}
		post := testutil.CreateTestPost(reportTestPostID, "other-user", models.PostTypeFeed)
		postRepo.On("GetByID", mock.Anything, reportTestPostID).Return(post, nil)
		reportRepo.On("CreatePostReport", mock.Anything, mock.AnythingOfType("*models.PostReport")).
			Return(fmt.Errorf("db error"))
		r := newReportRouter(t, reportRepo, postRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/posts/"+reportTestPostID+"/report",
			strings.NewReader(`{"reason":"spam"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- ReportComment ---

func TestReportHandler_ReportComment(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newReportRouter(t, &mocks.MockReportRepository{}, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/comments/"+reportTestCommentID+"/report",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		reportRepo := &mocks.MockReportRepository{}
		reportRepo.On("CreateCommentReport", mock.Anything, mock.AnythingOfType("*models.CommentReport")).Return(nil)
		r := newReportRouter(t, reportRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/comments/"+reportTestCommentID+"/report",
			strings.NewReader(`{"reason":"hate speech"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- ReportUser ---

func TestReportHandler_ReportUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		reportRepo := &mocks.MockReportRepository{}
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, reportTestTargetUID).Return(testutil.CreateTestUser(reportTestTargetUID, "target@test.com"), nil)
		reportRepo.On("CreateUserReport", mock.Anything, mock.AnythingOfType("*models.UserReport")).Return(nil)
		r := newReportRouter(t, reportRepo, &mocks.MockPostRepository{}, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/users/"+reportTestTargetUID+"/report",
			strings.NewReader(`{"reason":"harassment"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- ReportBusiness ---

func TestReportHandler_ReportBusiness(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		reportRepo := &mocks.MockReportRepository{}
		reportRepo.On("CreateBusinessReport", mock.Anything, mock.AnythingOfType("*models.BusinessReport")).Return(nil)
		r := newReportRouter(t, reportRepo, &mocks.MockPostRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/businesses/"+reportTestBusinessID+"/report",
			strings.NewReader(`{"reason":"fraud"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

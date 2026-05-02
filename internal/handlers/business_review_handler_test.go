package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	reviewTestUserID = "review-user-001"
	reviewTestBizID  = "review-biz-001"
	reviewTestRevID  = "review-rev-001"
)

func newReviewRouter(
	t *testing.T,
	reviewRepo *mocks.MockBusinessReviewRepository,
	bizRepo *mocks.MockBusinessRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewBusinessReviewService(reviewRepo, bizRepo, userRepo, nil, zap.NewNop())
	h := NewBusinessReviewHandler(svc, userRepo, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(reviewTestUserID, "review-sess-001")
	r := gin.New()

	authedGroup := r.Group("/api/v1")
	authedGroup.Use(authed)
	authedGroup.POST("/businesses/:business_id/reviews", h.SubmitReview)
	authedGroup.PUT("/businesses/:business_id/reviews/:review_id", h.UpdateReview)
	authedGroup.DELETE("/businesses/:business_id/reviews/:review_id", h.DeleteReview)
	authedGroup.GET("/businesses/:business_id/reviews/me", h.GetMyReview)
	authedGroup.PATCH("/admin/business-reviews/:review_id/hidden", h.SetHidden)

	// public endpoints — no auth middleware
	r.GET("/api/v1/businesses/:business_id/reviews", h.ListReviews)
	r.GET("/api/v1/businesses/:business_id/reviews/stats", h.GetStats)

	// noauth submit for unauthenticated test
	r.POST("/api/v1/noauth/businesses/:business_id/reviews", h.SubmitReview)
	return r
}

// --- SubmitReview ----------------------------------------------------------

func TestBusinessReviewHandler_SubmitReview(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		r := newReviewRouter(t,
			&mocks.MockBusinessReviewRepository{},
			&mocks.MockBusinessRepository{},
			&mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/noauth/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`{"rating":5}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newReviewRouter(t,
			&mocks.MockBusinessReviewRepository{},
			&mocks.MockBusinessRepository{},
			&mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rating out of range", func(t *testing.T) {
		r := newReviewRouter(t,
			&mocks.MockBusinessReviewRepository{},
			&mocks.MockBusinessRepository{},
			&mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`{"rating":6}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("self-review rejected", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, reviewTestBizID).
			Return(&models.BusinessProfile{ID: reviewTestBizID, UserID: reviewTestUserID}, nil)
		r := newReviewRouter(t, &mocks.MockBusinessReviewRepository{}, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`{"rating":5}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("business not found", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, reviewTestBizID).Return(nil, errors.New("nope"))
		r := newReviewRouter(t, &mocks.MockBusinessReviewRepository{}, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`{"rating":5}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.BusinessReview")).Return(nil)
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, reviewTestBizID).
			Return(&models.BusinessProfile{ID: reviewTestBizID, UserID: "owner-9"}, nil)
		r := newReviewRouter(t, reviewRepo, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews",
			strings.NewReader(`{"rating":5,"comment":"Great place"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- UpdateReview ----------------------------------------------------------

func TestBusinessReviewHandler_UpdateReview(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Update", mock.Anything, reviewTestRevID, reviewTestUserID, mock.Anything, mock.Anything).
			Return(nil, repositories.ErrReviewNotFound)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID,
			strings.NewReader(`{"rating":3}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("rating out of range", func(t *testing.T) {
		r := newReviewRouter(t,
			&mocks.MockBusinessReviewRepository{},
			&mocks.MockBusinessRepository{},
			&mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID,
			strings.NewReader(`{"rating":99}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		updated := &models.BusinessReview{
			ID: reviewTestRevID, BusinessProfileID: reviewTestBizID,
			UserID: reviewTestUserID, Rating: 3,
		}
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("Update", mock.Anything, reviewTestRevID, reviewTestUserID, mock.Anything, mock.Anything).
			Return(updated, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID,
			strings.NewReader(`{"rating":3,"comment":"updated"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- DeleteReview ----------------------------------------------------------

func TestBusinessReviewHandler_DeleteReview(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		userRepo := &mocks.MockUserRepository{}
		// non-admin path: GetByID returns regular user, allowAdmin=false
		userRepo.On("GetByID", mock.Anything, reviewTestUserID).
			Return(&models.User{ID: reviewTestUserID, Role: models.RoleUser}, nil)
		reviewRepo.On("Delete", mock.Anything, reviewTestRevID, reviewTestUserID, false).
			Return(repositories.ErrReviewNotFound)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("admin can delete any review", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, reviewTestUserID).
			Return(&models.User{ID: reviewTestUserID, Role: models.RoleAdmin}, nil)
		reviewRepo.On("Delete", mock.Anything, reviewTestRevID, reviewTestUserID, true).Return(nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		reviewRepo.AssertExpectations(t)
	})

	t.Run("regular user can delete own review", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, reviewTestUserID).
			Return(&models.User{ID: reviewTestUserID, Role: models.RoleUser}, nil)
		reviewRepo.On("Delete", mock.Anything, reviewTestRevID, reviewTestUserID, false).Return(nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/"+reviewTestRevID, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- ListReviews -----------------------------------------------------------

func TestBusinessReviewHandler_ListReviews(t *testing.T) {
	t.Run("public list hides moderated rows", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		// public route — no user_id in context, so isAdmin -> false -> includeHidden=false
		reviewRepo.On("ListByBusiness", mock.Anything, reviewTestBizID, false, 20, 0).
			Return([]*models.BusinessReviewWithAuthor{
				{BusinessReview: models.BusinessReview{ID: "r1", Rating: 4}},
			}, 1, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		reviewRepo.AssertExpectations(t)
	})

	t.Run("custom limit/offset query params", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("ListByBusiness", mock.Anything, reviewTestBizID, false, 5, 10).
			Return([]*models.BusinessReviewWithAuthor{}, 0, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews?limit=5&offset=10", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- GetMyReview -----------------------------------------------------------

func TestBusinessReviewHandler_GetMyReview(t *testing.T) {
	t.Run("returns null when none exists", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetByBusinessAndUser", mock.Anything, reviewTestBizID, reviewTestUserID).
			Return(nil, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/me", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns existing review", func(t *testing.T) {
		want := &models.BusinessReview{
			ID: reviewTestRevID, BusinessProfileID: reviewTestBizID,
			UserID: reviewTestUserID, Rating: 5,
		}
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetByBusinessAndUser", mock.Anything, reviewTestBizID, reviewTestUserID).
			Return(want, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/me", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), reviewTestRevID)
	})
}

// --- GetStats --------------------------------------------------------------

func TestBusinessReviewHandler_GetStats(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetStats", mock.Anything, reviewTestBizID).
			Return(nil, errors.New("db"))
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/stats", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("GetStats", mock.Anything, reviewTestBizID).
			Return(&models.BusinessReviewStats{
				BusinessProfileID: reviewTestBizID,
				AvgRating:         4.2, ReviewCount: 5,
				Distribution: [5]int{0, 0, 1, 2, 2},
			}, nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet,
			"/api/v1/businesses/"+reviewTestBizID+"/reviews/stats", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "avg_rating")
	})
}

// --- SetHidden --------------------------------------------------------------

func TestBusinessReviewHandler_SetHidden(t *testing.T) {
	t.Run("invalid hidden flag", func(t *testing.T) {
		r := newReviewRouter(t,
			&mocks.MockBusinessReviewRepository{},
			&mocks.MockBusinessRepository{},
			&mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch,
			"/api/v1/admin/business-reviews/"+reviewTestRevID+"/hidden?hidden=maybe", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("SetHidden", mock.Anything, reviewTestRevID, true).
			Return(repositories.ErrReviewNotFound)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch,
			"/api/v1/admin/business-reviews/"+reviewTestRevID+"/hidden?hidden=true", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		reviewRepo := &mocks.MockBusinessReviewRepository{}
		reviewRepo.On("SetHidden", mock.Anything, reviewTestRevID, true).Return(nil)
		r := newReviewRouter(t, reviewRepo, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch,
			"/api/v1/admin/business-reviews/"+reviewTestRevID+"/hidden?hidden=true", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

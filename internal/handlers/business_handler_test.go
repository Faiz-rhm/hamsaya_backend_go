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
	bizTestUserID = "biz-user-001"
	bizTestBizID  = "biz-biz-001"
)

func newBusinessRouter(
	t *testing.T,
	bizRepo *mocks.MockBusinessRepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewBusinessService(bizRepo, userRepo, nil, zap.NewNop())
	h := NewBusinessHandler(svc, nil, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(bizTestUserID, "biz-sess-001")
	r := gin.New()
	r.POST("/api/v1/businesses", authed, h.CreateBusiness)
	r.GET("/api/v1/businesses/:business_id", authed, h.GetBusiness)
	r.PUT("/api/v1/businesses/:business_id", authed, h.UpdateBusiness)
	r.DELETE("/api/v1/businesses/:business_id", authed, h.DeleteBusiness)
	r.GET("/api/v1/users/my/businesses", authed, h.GetMyBusinesses)

	r.POST("/api/v1/noauth/businesses", h.CreateBusiness)
	return r
}

// --- CreateBusiness ---

func TestBusinessHandler_CreateBusiness(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newBusinessRouter(t, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/businesses",
			strings.NewReader(`{"name":"My Shop"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newBusinessRouter(t, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/businesses",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing name", func(t *testing.T) {
		r := newBusinessRouter(t, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/businesses",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.BusinessProfile")).Return(nil)
		bizRepo.On("GetByID", mock.Anything, mock.Anything).
			Return(testutil.CreateTestBusiness(bizTestBizID, bizTestUserID, "My Shop"), nil)
		bizRepo.On("GetHoursByBusinessID", mock.Anything, mock.Anything).Return([]*models.BusinessHours{}, nil).Maybe()
		bizRepo.On("GetCategoriesByBusinessID", mock.Anything, mock.Anything).Return([]*models.BusinessCategory{}, nil).Maybe()
		bizRepo.On("GetAttachmentsByBusinessID", mock.Anything, mock.Anything).Return([]*models.BusinessAttachment{}, nil).Maybe()
		bizRepo.On("IsFollowing", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
		bizRepo.On("IncrementViews", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/businesses",
			strings.NewReader(`{"name":"My Shop"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- GetBusiness ---

func TestBusinessHandler_GetBusiness(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(nil, fmt.Errorf("not found"))
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/businesses/"+bizTestBizID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		biz := testutil.CreateTestBusiness(bizTestBizID, bizTestUserID, "My Shop")
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(biz, nil)
		bizRepo.On("GetHoursByBusinessID", mock.Anything, bizTestBizID).Return([]*models.BusinessHours{}, nil).Maybe()
		bizRepo.On("GetCategoriesByBusinessID", mock.Anything, bizTestBizID).Return([]*models.BusinessCategory{}, nil).Maybe()
		bizRepo.On("GetAttachmentsByBusinessID", mock.Anything, bizTestBizID).Return([]*models.BusinessAttachment{}, nil).Maybe()
		bizRepo.On("IsFollowing", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
		bizRepo.On("IncrementViews", mock.Anything, mock.Anything).Return(nil).Maybe()
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/businesses/"+bizTestBizID, nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- UpdateBusiness ---

func TestBusinessHandler_UpdateBusiness(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newBusinessRouter(t, &mocks.MockBusinessRepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/businesses/"+bizTestBizID,
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("business not found", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(nil, fmt.Errorf("not found"))
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/businesses/"+bizTestBizID,
			strings.NewReader(`{"name":"Updated"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- DeleteBusiness ---

func TestBusinessHandler_DeleteBusiness(t *testing.T) {
	t.Run("business not found", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(nil, fmt.Errorf("not found"))
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/businesses/"+bizTestBizID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("not owner", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		biz := testutil.CreateTestBusiness(bizTestBizID, "other-owner", "My Shop")
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(biz, nil)
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/businesses/"+bizTestBizID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		biz := testutil.CreateTestBusiness(bizTestBizID, bizTestUserID, "My Shop")
		bizRepo.On("GetByID", mock.Anything, bizTestBizID).Return(biz, nil)
		bizRepo.On("Delete", mock.Anything, bizTestBizID).Return(nil)
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/businesses/"+bizTestBizID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		bizRepo.AssertExpectations(t)
	})
}

// --- GetMyBusinesses ---

func TestBusinessHandler_GetMyBusinesses(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		bizRepo := &mocks.MockBusinessRepository{}
		bizRepo.On("GetByUserID", mock.Anything, bizTestUserID, 20, 0).
			Return([]*models.BusinessProfile{}, nil)
		r := newBusinessRouter(t, bizRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/my/businesses", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

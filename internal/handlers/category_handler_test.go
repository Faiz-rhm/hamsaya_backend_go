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
	catTestUserID = "cat-user-001"
	catTestCatID  = "cat-cat-001"
)

func newCategoryRouter(t *testing.T, catRepo *mocks.MockCategoryRepository) *gin.Engine {
	t.Helper()
	svc := services.NewCategoryService(catRepo, zap.NewNop())
	h := NewCategoryHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(catTestUserID, "cat-sess-001")
	r := gin.New()
	r.GET("/api/v1/categories", authed, h.ListCategories)
	r.GET("/api/v1/categories/:category_id", authed, h.GetCategory)
	r.POST("/api/v1/admin/categories", authed, h.CreateCategory)
	r.PUT("/api/v1/admin/categories/:category_id", authed, h.UpdateCategory)
	r.DELETE("/api/v1/admin/categories/:category_id", authed, h.DeleteCategory)
	r.GET("/api/v1/admin/categories", authed, h.GetAllCategories)
	return r
}

// --- ListCategories ---

func TestCategoryHandler_ListCategories(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetActiveCategories", mock.Anything).Return([]*models.SellCategory{}, nil)
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
		catRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetActiveCategories", mock.Anything).Return(nil, fmt.Errorf("db error"))
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetCategory ---

func TestCategoryHandler_GetCategory(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetByID", mock.Anything, catTestCatID).Return(nil, fmt.Errorf("not found"))
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories/"+catTestCatID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		cat := &models.SellCategory{ID: catTestCatID, Name: "Electronics"}
		catRepo.On("GetByID", mock.Anything, catTestCatID).Return(cat, nil)
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories/"+catTestCatID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- CreateCategory ---

func TestCategoryHandler_CreateCategory(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newCategoryRouter(t, &mocks.MockCategoryRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/categories",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		r := newCategoryRouter(t, &mocks.MockCategoryRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/categories",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.SellCategory")).Return(nil)
		catRepo.On("GetByID", mock.Anything, mock.Anything).
			Return(&models.SellCategory{ID: "new-cat", Name: "Books"}, nil)
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/categories",
			strings.NewReader(`{"name":"Books","icon":{"name":"book","library":"material"},"color":"#FF5733"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- UpdateCategory ---

func TestCategoryHandler_UpdateCategory(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newCategoryRouter(t, &mocks.MockCategoryRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/admin/categories/"+catTestCatID,
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetByID", mock.Anything, catTestCatID).Return(nil, fmt.Errorf("not found"))
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/admin/categories/"+catTestCatID,
			strings.NewReader(`{"name":"Updated"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- DeleteCategory ---

func TestCategoryHandler_DeleteCategory(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetByID", mock.Anything, catTestCatID).Return(&models.SellCategory{ID: catTestCatID}, nil)
		catRepo.On("Delete", mock.Anything, catTestCatID).Return(fmt.Errorf("db error"))
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/categories/"+catTestCatID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		catRepo := &mocks.MockCategoryRepository{}
		catRepo.On("GetByID", mock.Anything, catTestCatID).Return(&models.SellCategory{ID: catTestCatID}, nil)
		catRepo.On("Delete", mock.Anything, catTestCatID).Return(nil)
		r := newCategoryRouter(t, catRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/admin/categories/"+catTestCatID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		catRepo.AssertExpectations(t)
	})
}

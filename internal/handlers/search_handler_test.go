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

const searchTestUserID = "search-user-001"

func newSearchRouter(
	t *testing.T,
	searchRepo *mocks.MockSearchRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewSearchService(
		searchRepo,
		&mocks.MockPostRepository{},
		&mocks.MockUserRepository{},
		&mocks.MockBusinessRepository{},
		&mocks.MockCategoryRepository{},
		&mocks.MockRelationshipsRepository{},
		zap.NewNop(),
	)
	h := NewSearchHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := authContextMiddleware(searchTestUserID, "search-sess-001")
	r := gin.New()
	r.GET("/api/v1/search", authed, h.Search)
	r.GET("/api/v1/search/posts", authed, h.SearchPosts)
	r.GET("/api/v1/search/users", authed, h.SearchUsers)
	r.GET("/api/v1/search/businesses", authed, h.SearchBusinesses)
	r.GET("/api/v1/discover", authed, h.Discover)
	return r
}

// --- Search ---

func TestSearchHandler_Search(t *testing.T) {
	t.Run("missing query param", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		searchRepo.On("SearchPosts", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.Post{}, nil)
		searchRepo.On("SearchUsers", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.Profile{}, nil)
		searchRepo.On("SearchBusinesses", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.BusinessProfile{}, nil)

		r := newSearchRouter(t, searchRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search?query=test", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})

	t.Run("search repo error", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		searchRepo.On("SearchPosts", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return(nil, fmt.Errorf("db error"))
		searchRepo.On("SearchUsers", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.Profile{}, nil)
		searchRepo.On("SearchBusinesses", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.BusinessProfile{}, nil)

		r := newSearchRouter(t, searchRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search?query=test", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- SearchPosts ---

func TestSearchHandler_SearchPosts(t *testing.T) {
	t.Run("missing query", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/posts", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		searchRepo.On("SearchPosts", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.Post{}, nil)

		r := newSearchRouter(t, searchRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/posts?query=hello", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- SearchUsers ---

func TestSearchHandler_SearchUsers(t *testing.T) {
	t.Run("missing query", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/users", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		searchRepo.On("SearchUsers", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return([]*models.Profile{}, nil)

		r := newSearchRouter(t, searchRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/users?query=alice", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- SearchBusinesses ---

func TestSearchHandler_SearchBusinesses(t *testing.T) {
	t.Run("missing query", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/search/businesses", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Discover ---

func TestSearchHandler_Discover(t *testing.T) {
	t.Run("missing lat/lng/radius", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/discover", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid latitude", func(t *testing.T) {
		r := newSearchRouter(t, &mocks.MockSearchRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/discover?latitude=bad&longitude=69.1&radius_km=10", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		searchRepo.On("GetDiscoverPosts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]*models.Post{}, nil)
		searchRepo.On("GetDiscoverBusinesses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]*models.BusinessProfile{}, nil)

		r := newSearchRouter(t, searchRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/discover?latitude=34.5&longitude=69.1&radius_km=10", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

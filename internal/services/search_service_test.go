package services

import (
	"context"
	"errors"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestSearchService(
	searchRepo *mocks.MockSearchRepository,
	postRepo *mocks.MockPostRepository,
	userRepo *mocks.MockUserRepository,
	businessRepo *mocks.MockBusinessRepository,
	categoryRepo *mocks.MockCategoryRepository,
	relRepo *mocks.MockRelationshipsRepository,
) *SearchService {
	return NewSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo, zap.NewNop())
}

func TestSearchService_Search(t *testing.T) {
	t.Run("search posts success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		posts := []*models.Post{{ID: "p-1", Type: models.PostTypeFeed}}
		searchRepo.On("SearchPosts", mock.Anything, mock.AnythingOfType("*models.SearchFilter")).
			Return(posts, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypePosts, Limit: 10,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		assert.Len(t, resp.Posts, 1)
		searchRepo.AssertExpectations(t)
	})

	t.Run("search posts error", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("SearchPosts", mock.Anything, mock.Anything).
			Return(nil, errors.New("db error"))

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypePosts,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("search users success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		fn, ln := "Test", "User"
		profiles := []*models.Profile{{ID: "user-1", FirstName: &fn, LastName: &ln}}
		searchRepo.On("SearchUsers", mock.Anything, mock.Anything).Return(profiles, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypeUsers,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		assert.Len(t, resp.Users, 1)
	})

	t.Run("search businesses success", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		businesses := []*models.BusinessProfile{{ID: "biz-1", Name: "Test Biz"}}
		searchRepo.On("SearchBusinesses", mock.Anything, mock.Anything).Return(businesses, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypeBusinesses,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		assert.Len(t, resp.Businesses, 1)
	})

	t.Run("search all types", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("SearchPosts", mock.Anything, mock.Anything).
			Return([]*models.Post{{ID: "p-1", Type: models.PostTypeFeed}}, nil)
		searchRepo.On("SearchUsers", mock.Anything, mock.Anything).
			Return([]*models.Profile{}, nil)
		searchRepo.On("SearchBusinesses", mock.Anything, mock.Anything).
			Return([]*models.BusinessProfile{}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypeAll,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		searchRepo.AssertExpectations(t)
	})

	t.Run("default limit applied", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("SearchPosts", mock.Anything, mock.MatchedBy(func(f *models.SearchFilter) bool {
			return f.Limit == 20
		})).Return([]*models.Post{}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		// Limit 0 — should default to 20
		resp, err := svc.Search(context.Background(), nil, &models.SearchRequest{
			Query: "test", Type: models.SearchTypePosts, Limit: 0,
		})

		require.NoError(t, err)
		assert.NotNil(t, resp)
		searchRepo.AssertExpectations(t)
	})
}

func TestSearchService_Discover(t *testing.T) {
	t.Run("all filter returns posts and businesses", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("GetDiscoverPosts", mock.Anything, 34.5, 69.2, 10.0, (*models.PostType)(nil), 100).
			Return([]*models.Post{{ID: "p-1", Type: models.PostTypeEvent}}, nil)
		searchRepo.On("GetDiscoverBusinesses", mock.Anything, 34.5, 69.2, 10.0, 100).
			Return([]*models.BusinessProfile{{ID: "biz-1", Name: "Biz"}}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Discover(context.Background(), nil, &models.DiscoverRequest{
			Latitude: 34.5, Longitude: 69.2, RadiusKm: 10.0,
			Filter: models.DiscoverFilterAll,
		})

		require.NoError(t, err)
		assert.Equal(t, 2, resp.Total)
		assert.Len(t, resp.Posts, 1)
		assert.Len(t, resp.Businesses, 1)
		searchRepo.AssertExpectations(t)
	})

	t.Run("event filter — only event posts", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		eventType := models.PostTypeEvent
		searchRepo.On("GetDiscoverPosts", mock.Anything, 34.5, 69.2, 5.0, &eventType, 100).
			Return([]*models.Post{}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Discover(context.Background(), nil, &models.DiscoverRequest{
			Latitude: 34.5, Longitude: 69.2, RadiusKm: 5.0,
			Filter: models.DiscoverFilterEvent,
		})

		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
		searchRepo.AssertExpectations(t)
		searchRepo.AssertNotCalled(t, "GetDiscoverBusinesses")
	})

	t.Run("business filter — only businesses", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("GetDiscoverBusinesses", mock.Anything, 34.5, 69.2, 5.0, 100).
			Return([]*models.BusinessProfile{}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		resp, err := svc.Discover(context.Background(), nil, &models.DiscoverRequest{
			Latitude: 34.5, Longitude: 69.2, RadiusKm: 5.0,
			Filter: models.DiscoverFilterBusiness,
		})

		require.NoError(t, err)
		assert.Equal(t, 0, resp.Total)
		searchRepo.AssertExpectations(t)
		searchRepo.AssertNotCalled(t, "GetDiscoverPosts")
	})

	t.Run("default limit 100 applied", func(t *testing.T) {
		searchRepo := &mocks.MockSearchRepository{}
		postRepo := &mocks.MockPostRepository{}
		userRepo := new(mocks.MockUserRepository)
		businessRepo := &mocks.MockBusinessRepository{}
		categoryRepo := &mocks.MockCategoryRepository{}
		relRepo := &mocks.MockRelationshipsRepository{}

		searchRepo.On("GetDiscoverPosts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 100).
			Return([]*models.Post{}, nil)
		searchRepo.On("GetDiscoverBusinesses", mock.Anything, mock.Anything, mock.Anything, mock.Anything, 100).
			Return([]*models.BusinessProfile{}, nil)

		svc := newTestSearchService(searchRepo, postRepo, userRepo, businessRepo, categoryRepo, relRepo)
		// Limit 0 — should default to 100
		resp, err := svc.Discover(context.Background(), nil, &models.DiscoverRequest{
			Latitude: 34.5, Longitude: 69.2, RadiusKm: 5.0, Limit: 0,
		})

		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

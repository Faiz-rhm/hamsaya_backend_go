package repositories_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newSearchRepo(pool *testutil.MockPool) repositories.SearchRepository {
	return repositories.NewSearchRepository(testutil.NewTestDB(pool))
}

func TestSearchRepository_SearchPosts_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	filter := &models.SearchFilter{Query: "test", Limit: 10}
	posts, err := repo.SearchPosts(context.Background(), filter)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestSearchRepository_SearchPosts_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	filter := &models.SearchFilter{Query: "test", Limit: 10}
	_, err := repo.SearchPosts(context.Background(), filter)
	require.Error(t, err)
}

func TestSearchRepository_SearchUsers_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	filter := &models.SearchFilter{Query: "john", Limit: 10}
	users, err := repo.SearchUsers(context.Background(), filter)
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestSearchRepository_SearchUsers_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, err := repo.SearchUsers(context.Background(), &models.SearchFilter{Query: "john", Limit: 10})
	require.Error(t, err)
}

func TestSearchRepository_SearchBusinesses_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	filter := &models.SearchFilter{Query: "cafe", Limit: 10}
	businesses, err := repo.SearchBusinesses(context.Background(), filter)
	require.NoError(t, err)
	assert.Empty(t, businesses)
}

func TestSearchRepository_GetDiscoverPosts_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	posts, err := repo.GetDiscoverPosts(context.Background(), 34.5, 69.2, 10.0, nil, 20)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestSearchRepository_GetDiscoverPosts_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, err := repo.GetDiscoverPosts(context.Background(), 34.5, 69.2, 10.0, nil, 20)
	require.Error(t, err)
}

func TestSearchRepository_GetDiscoverBusinesses_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newSearchRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	businesses, err := repo.GetDiscoverBusinesses(context.Background(), 34.5, 69.2, 5.0, 10)
	require.NoError(t, err)
	assert.Empty(t, businesses)
}

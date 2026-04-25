package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newFanoutRepo(pool *testutil.MockPool) repositories.FanoutRepository {
	return repositories.NewFanoutRepository(testutil.NewTestDB(pool))
}

func TestFanoutRepository_CountFollowers_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 42
			return nil
		}))

	count, err := repo.CountFollowers(context.Background(), "author-1")
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestFanoutRepository_CountFollowers_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("db error")))

	_, err := repo.CountFollowers(context.Background(), "author-1")
	require.Error(t, err)
}

func TestFanoutRepository_GetFollowerIDs_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(
			func(dest ...any) error {
				*dest[0].(*string) = "follower-1"
				return nil
			},
			func(dest ...any) error {
				*dest[0].(*string) = "follower-2"
				return nil
			},
		), nil)

	ids, err := repo.GetFollowerIDs(context.Background(), "author-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"follower-1", "follower-2"}, ids)
}

func TestFanoutRepository_GetFollowerIDs_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, err := repo.GetFollowerIDs(context.Background(), "author-1")
	require.Error(t, err)
}

func TestFanoutRepository_InsertFeedEntries_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("SendBatch", mock.Anything, mock.Anything).
		Return(testutil.NewMockBatchResults(nil))

	err := repo.InsertFeedEntries(context.Background(), "post-1", []string{"f1", "f2"})
	require.NoError(t, err)
}

func TestFanoutRepository_InsertFeedEntries_EmptyFollowers(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)
	// no DB calls expected for empty slice
	err := repo.InsertFeedEntries(context.Background(), "post-1", []string{})
	require.NoError(t, err)
}

func TestFanoutRepository_GetPersonalizedFeed_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "post-1"
			*dest[1].(*time.Time) = now
			return nil
		}), nil)

	ids, cursor, err := repo.GetPersonalizedFeed(context.Background(), "user-1", nil, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"post-1"}, ids)
	assert.NotNil(t, cursor)
}

func TestFanoutRepository_GetPersonalizedFeed_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFanoutRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	ids, cursor, err := repo.GetPersonalizedFeed(context.Background(), "user-1", nil, 10)
	require.NoError(t, err)
	assert.Empty(t, ids)
	assert.Nil(t, cursor)
}

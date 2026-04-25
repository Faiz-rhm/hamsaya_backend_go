package repositories_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newRelRepo(pool *testutil.MockPool) repositories.RelationshipsRepository {
	return repositories.NewRelationshipsRepository(testutil.NewTestDB(pool))
}

func TestRelationshipsRepository_FollowUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.FollowUser(context.Background(), "user-1", "user-2")

		require.NoError(t, err)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.FollowUser(context.Background(), "user-1", "user-2")

		require.Error(t, err)
	})
}

func TestRelationshipsRepository_UnfollowUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.UnfollowUser(context.Background(), "user-1", "user-2")

		require.NoError(t, err)
	})
}

func TestRelationshipsRepository_IsFollowing(t *testing.T) {
	t.Run("returns true", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if b, ok := dest[0].(*bool); ok {
				*b = true
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		following, err := repo.IsFollowing(context.Background(), "user-1", "user-2")

		require.NoError(t, err)
		assert.True(t, following)
	})

	t.Run("returns false", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if b, ok := dest[0].(*bool); ok {
				*b = false
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		following, err := repo.IsFollowing(context.Background(), "user-1", "user-2")

		require.NoError(t, err)
		assert.False(t, following)
	})
}

func TestRelationshipsRepository_GetFollowersCount(t *testing.T) {
	t.Run("returns count", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if p, ok := dest[0].(*int); ok {
				*p = 42
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		count, err := repo.GetFollowersCount(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, 42, count)
	})
}

func TestRelationshipsRepository_GetFollowingCount(t *testing.T) {
	t.Run("returns count", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if p, ok := dest[0].(*int); ok {
				*p = 10
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		count, err := repo.GetFollowingCount(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, 10, count)
	})
}

func TestRelationshipsRepository_BlockUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newRelRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.BlockUser(context.Background(), "user-1", "user-2")

		require.NoError(t, err)
	})
}

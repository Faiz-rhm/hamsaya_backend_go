package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newPostRepo(pool *testutil.MockPool) repositories.PostRepository {
	return repositories.NewPostRepository(testutil.NewTestDB(pool))
}

func strPtr(s string) *string { return &s }

func testPost() *models.Post {
	now := time.Now()
	return &models.Post{
		ID:        "post-1",
		UserID:    strPtr("user-1"),
		Type:      models.PostTypeFeed,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestPostRepository_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Create(context.Background(), testPost())

		require.NoError(t, err)
		pool.AssertExpectations(t)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Create(context.Background(), testPost())

		require.Error(t, err)
	})
}

func TestPostRepository_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Delete(context.Background(), "post-1")

		require.NoError(t, err)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Delete(context.Background(), "post-1")

		require.Error(t, err)
	})
}

func TestPostRepository_LikePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.LikePost(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
	})
}

func TestPostRepository_UnlikePost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.UnlikePost(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
	})
}

func TestPostRepository_IsLikedByUser(t *testing.T) {
	t.Run("returns true when liked", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if b, ok := dest[0].(*bool); ok {
				*b = true
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		liked, err := repo.IsLikedByUser(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
		assert.True(t, liked)
	})

	t.Run("returns false when not liked", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if b, ok := dest[0].(*bool); ok {
				*b = false
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		liked, err := repo.IsLikedByUser(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
		assert.False(t, liked)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(pgx.ErrNoRows))

		_, err := repo.IsLikedByUser(context.Background(), "user-1", "post-1")

		require.Error(t, err)
	})
}

func TestPostRepository_BookmarkPost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.BookmarkPost(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
	})
}

func TestPostRepository_UnbookmarkPost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.UnbookmarkPost(context.Background(), "user-1", "post-1")

		require.NoError(t, err)
	})
}

func TestPostRepository_CreateAttachment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newPostRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		att := &models.Attachment{
			ID:        "att-1",
			PostID:    "post-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.CreateAttachment(context.Background(), att)

		require.NoError(t, err)
	})
}

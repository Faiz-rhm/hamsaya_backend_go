package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newCommentRepo(pool *testutil.MockPool) repositories.CommentRepository {
	return repositories.NewCommentRepository(testutil.NewTestDB(pool))
}

func testComment() *models.PostComment {
	now := time.Now()
	return &models.PostComment{
		ID:        "comment-1",
		PostID:    "post-1",
		UserID:    "user-1",
		Text:      "Test comment",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCommentRepository_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Create(context.Background(), testComment())

		require.NoError(t, err)
		pool.AssertExpectations(t)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Create(context.Background(), testComment())

		require.Error(t, err)
	})
}

func TestCommentRepository_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.Delete(context.Background(), "comment-1")

		require.NoError(t, err)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Delete(context.Background(), "comment-1")

		require.Error(t, err)
	})
}

func TestCommentRepository_CountByPostID(t *testing.T) {
	t.Run("returns count", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if p, ok := dest[0].(*int); ok {
				*p = 3
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		count, err := repo.CountByPostID(context.Background(), "post-1")

		require.NoError(t, err)
		require.Equal(t, 3, count)
	})
}

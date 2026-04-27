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

func TestCommentRepository_GetByUserID(t *testing.T) {
	t.Run("returns user-authored comments", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		now := time.Now()
		rows := testutil.NewFuncRows(
			func(dest ...any) error {
				// id, post_id, user_id, business_id, parent_comment_id, text,
				// location, total_likes, total_replies, created_at, updated_at,
				// deleted_at, mentioned_user_ids
				*dest[0].(*string) = "c1"
				*dest[1].(*string) = "p1"
				*dest[2].(*string) = "user-target"
				*dest[5].(*string) = "first"
				*dest[7].(*int) = 0
				*dest[8].(*int) = 0
				*dest[9].(*time.Time) = now
				*dest[10].(*time.Time) = now
				return nil
			},
			func(dest ...any) error {
				*dest[0].(*string) = "c2"
				*dest[1].(*string) = "p2"
				*dest[2].(*string) = "user-target"
				*dest[5].(*string) = "second"
				*dest[9].(*time.Time) = now
				*dest[10].(*time.Time) = now
				return nil
			},
		)
		pool.On("Query", mock.Anything, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything).Return(rows, nil)

		got, err := repo.GetByUserID(context.Background(), "user-target", 100, 0)

		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Equal(t, "c1", got[0].ID)
		require.Equal(t, "c2", got[1].ID)
		require.Equal(t, "user-target", got[0].UserID)
	})

	t.Run("propagates query error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newCommentRepo(pool)

		pool.On("Query", mock.Anything, mock.AnythingOfType("string"),
			mock.Anything, mock.Anything, mock.Anything).
			Return((*testutil.FuncRows)(nil), fmt.Errorf("db down"))

		_, err := repo.GetByUserID(context.Background(), "user-target", 100, 0)
		require.Error(t, err)
	})
}

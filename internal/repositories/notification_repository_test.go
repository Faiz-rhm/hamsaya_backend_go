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

func newNotifRepo(pool *testutil.MockPool) repositories.NotificationRepository {
	return repositories.NewNotificationRepository(testutil.NewTestDB(pool))
}

func testNotification() *models.Notification {
	return &models.Notification{
		ID:        "notif-1",
		UserID:    "user-1",
		Type:      models.NotificationTypeLike,
		Read:      false,
		CreatedAt: time.Now(),
	}
}

func TestNotificationRepository_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Create(context.Background(), testNotification())

		require.NoError(t, err)
		pool.AssertExpectations(t)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Create(context.Background(), testNotification())

		require.Error(t, err)
	})
}

func TestNotificationRepository_GetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		now := time.Now()
		row := testutil.NewMockRow(func(dest ...any) error {
			// Scan: id, user_id, type, title, message, data([]byte), read, created_at
			assigns := []any{
				"notif-1", "user-1", string(models.NotificationTypeLike),
				(*string)(nil), (*string)(nil), []byte("{}"), false, now,
			}
			for i, d := range dest {
				if i < len(assigns) {
					testutil.AssignValue(d, assigns[i])
				}
			}
			return nil
		})

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		n, err := repo.GetByID(context.Background(), "notif-1")

		require.NoError(t, err)
		assert.Equal(t, "notif-1", n.ID)
	})

	t.Run("not found", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(pgx.ErrNoRows))

		_, err := repo.GetByID(context.Background(), "missing")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestNotificationRepository_MarkAsRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.MarkAsRead(context.Background(), "notif-1")

		require.NoError(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.MarkAsRead(context.Background(), "missing")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestNotificationRepository_MarkAllAsRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.MarkAllAsRead(context.Background(), "user-1")

		require.NoError(t, err)
	})
}

func TestNotificationRepository_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("DELETE 1"), nil)

		err := repo.Delete(context.Background(), "notif-1")

		require.NoError(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("DELETE 0"), nil)

		err := repo.Delete(context.Background(), "missing")

		require.Error(t, err)
	})
}

func TestNotificationRepository_GetUnreadCount(t *testing.T) {
	t.Run("returns count", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if p, ok := dest[0].(*int); ok {
				*p = 5
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		count, err := repo.GetUnreadCount(context.Background(), "user-1", nil)

		require.NoError(t, err)
		assert.Equal(t, 5, count)
	})

	t.Run("returns count with businessID filter", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			if p, ok := dest[0].(*int); ok {
				*p = 2
			}
			return nil
		})
		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(row)

		bizID := "biz-1"
		count, err := repo.GetUnreadCount(context.Background(), "user-1", &bizID)

		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newNotifRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(fmt.Errorf("db error")))

		_, err := repo.GetUnreadCount(context.Background(), "user-1", nil)

		require.Error(t, err)
	})
}

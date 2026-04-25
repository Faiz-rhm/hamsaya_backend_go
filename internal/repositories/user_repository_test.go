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

func newUserRepo(pool *testutil.MockPool) repositories.UserRepository {
	return repositories.NewUserRepository(testutil.NewTestDB(pool))
}

func TestUserRepository_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		user := testutil.CreateTestUser("u-1", "test@example.com")
		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.Create(context.Background(), user)

		require.NoError(t, err)
		pool.AssertExpectations(t)
	})

	t.Run("duplicate email returns friendly error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pgErr := &pgconn.PgError{Code: "23505"}
		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, pgErr)

		err := repo.Create(context.Background(), testutil.CreateTestUser("u-2", "dup@example.com"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("propagates DB error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("connection refused"))

		err := repo.Create(context.Background(), testutil.CreateTestUser("u-3", "err@example.com"))

		require.Error(t, err)
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			// Scan order: id, email, phone, password_hash, email_verified,
			//             phone_verified, mfa_enabled, role, oauth_provider,
			//             oauth_provider_id, last_login_at, failed_login_attempts,
			//             locked_until, created_at, updated_at, deleted_at
			assigns := []any{
				"u-1", "test@example.com", (*string)(nil), (*string)(nil),
				true, false, false, "user", (*string)(nil), (*string)(nil),
				(*time.Time)(nil), 0, (*time.Time)(nil),
				now, now, (*time.Time)(nil),
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

		user, err := repo.GetByID(context.Background(), "u-1")

		require.NoError(t, err)
		assert.Equal(t, "u-1", user.ID)
		assert.Equal(t, "test@example.com", user.Email)
	})

	t.Run("not found returns error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(pgx.ErrNoRows))

		user, err := repo.GetByID(context.Background(), "missing")

		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("propagates DB error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(fmt.Errorf("timeout")))

		_, err := repo.GetByID(context.Background(), "u-1")

		require.Error(t, err)
	})
}

func TestUserRepository_GetByEmail(t *testing.T) {
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		row := testutil.NewMockRow(func(dest ...any) error {
			assigns := []any{
				"u-1", "test@example.com", (*string)(nil), (*string)(nil),
				true, false, false, "user", (*string)(nil), (*string)(nil),
				(*time.Time)(nil), 0, (*time.Time)(nil),
				now, now, (*time.Time)(nil),
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

		user, err := repo.GetByEmail(context.Background(), "test@example.com")

		require.NoError(t, err)
		assert.Equal(t, "test@example.com", user.Email)
	})

	t.Run("not found", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(testutil.ErrRow(pgx.ErrNoRows))

		_, err := repo.GetByEmail(context.Background(), "nobody@example.com")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.Update(context.Background(), testutil.CreateTestUser("u-1", "u@example.com"))

		require.NoError(t, err)
	})

	t.Run("propagates error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("db error"))

		err := repo.Update(context.Background(), testutil.CreateTestUser("u-1", "u@example.com"))

		require.Error(t, err)
	})
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		err := repo.UpdateLastLogin(context.Background(), "u-1")

		require.NoError(t, err)
	})
}

func TestUserRepository_SoftDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		tag := pgconn.NewCommandTag("DELETE 1")
		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(tag, nil)

		err := repo.SoftDelete(context.Background(), "u-1")

		require.NoError(t, err)
	})

	t.Run("not found returns error", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		tag := pgconn.NewCommandTag("DELETE 0")
		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(tag, nil)

		err := repo.SoftDelete(context.Background(), "missing")

		require.Error(t, err)
	})
}

func TestUserRepository_CreateProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pool := new(testutil.MockPool)
		repo := newUserRepo(pool)

		pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		profile := &models.Profile{
			ID:        "u-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.CreateProfile(context.Background(), profile)

		require.NoError(t, err)
	})
}

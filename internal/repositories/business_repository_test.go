package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/testutil"
)

func newBusinessRepo(pool *testutil.MockPool) repositories.BusinessRepository {
	return repositories.NewBusinessRepository(testutil.NewTestDB(pool))
}

func TestBusinessRepository_Create_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	business := &models.BusinessProfile{
		ID: "biz-1", UserID: "user-1", Name: "Cafe Kabul",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	err := repo.Create(context.Background(), business)
	require.NoError(t, err)
}

func TestBusinessRepository_Create_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.Create(context.Background(), &models.BusinessProfile{
		ID: "biz-1", UserID: "user-1", Name: "Test",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	require.Error(t, err)
}

func TestBusinessRepository_Delete_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.Delete(context.Background(), "biz-1")
	require.NoError(t, err)
}

func TestBusinessRepository_Follow_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	err := repo.Follow(context.Background(), "biz-1", "user-1")
	require.NoError(t, err)
}

func TestBusinessRepository_Unfollow_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 1"), nil)

	err := repo.Unfollow(context.Background(), "biz-1", "user-1")
	require.NoError(t, err)
}

func TestBusinessRepository_IsFollowing_True(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		}))

	ok, err := repo.IsFollowing(context.Background(), "biz-1", "user-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBusinessRepository_RemoveCategories_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 3"), nil)

	err := repo.RemoveCategories(context.Background(), "biz-1")
	require.NoError(t, err)
}

func TestBusinessRepository_DeleteHoursByBusinessID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 7"), nil)

	err := repo.DeleteHoursByBusinessID(context.Background(), "biz-1")
	require.NoError(t, err)
}

func TestBusinessRepository_IncrementViews_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.IncrementViews(context.Background(), "biz-1")
	require.NoError(t, err)
}

func TestBusinessRepository_GetCategoriesByBusinessID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "cat-1"
			*dest[1].(*string) = "Food"
			*dest[2].(*bool) = true
			*dest[3].(*time.Time) = now
			return nil
		}), nil)

	cats, err := repo.GetCategoriesByBusinessID(context.Background(), "biz-1")
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "Food", cats[0].Name)
}

func TestBusinessRepository_GetHoursByBusinessID_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newBusinessRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	hours, err := repo.GetHoursByBusinessID(context.Background(), "biz-1")
	require.NoError(t, err)
	assert.Empty(t, hours)
}

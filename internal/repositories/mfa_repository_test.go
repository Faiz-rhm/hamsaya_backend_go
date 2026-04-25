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

func newMFARepo(pool *testutil.MockPool) repositories.MFARepository {
	return repositories.NewMFARepository(testutil.NewTestDB(pool))
}

func TestMFARepository_CreateFactor_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	secretKey := "TOTP_SECRET"
	factor := &models.MFAFactor{
		ID: "factor-1", UserID: "user-1", Type: "TOTP",
		SecretKey: &secretKey, Status: "unverified",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	err := repo.CreateFactor(context.Background(), factor)
	require.NoError(t, err)
}

func TestMFARepository_CreateFactor_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	secretKey := "S"
	err := repo.CreateFactor(context.Background(), &models.MFAFactor{
		ID: "f1", UserID: "u1", Type: "TOTP", SecretKey: &secretKey,
		Status: "unverified", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	require.Error(t, err)
}

func TestMFARepository_GetFactorByID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	now := time.Now()
	secretKey := "SECRET"
	factorID := "ext-factor-id"

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "factor-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "TOTP"
			*dest[3].(**string) = &secretKey
			*dest[4].(**string) = &factorID
			*dest[5].(*string) = "verified"
			*dest[6].(*time.Time) = now
			*dest[7].(*time.Time) = now
			*dest[8].(**time.Time) = nil
			return nil
		}))

	factor, err := repo.GetFactorByID(context.Background(), "factor-1")
	require.NoError(t, err)
	assert.Equal(t, "factor-1", factor.ID)
	assert.Equal(t, "TOTP", factor.Type)
	assert.Equal(t, "verified", factor.Status)
}

func TestMFARepository_GetFactorByID_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	_, err := repo.GetFactorByID(context.Background(), "not-exist")
	require.Error(t, err)
}

func TestMFARepository_GetFactorsByUserID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	now := time.Now()
	secretKey := "SECRET"

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "factor-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "TOTP"
			*dest[3].(**string) = &secretKey
			*dest[4].(**string) = nil
			*dest[5].(*string) = "verified"
			*dest[6].(*time.Time) = now
			*dest[7].(*time.Time) = now
			*dest[8].(**time.Time) = nil
			return nil
		}), nil)

	factors, err := repo.GetFactorsByUserID(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, factors, 1)
}

func TestMFARepository_UpdateFactorStatus_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateFactorStatus(context.Background(), "factor-1", "verified")
	require.NoError(t, err)
}

func TestMFARepository_DeleteFactor_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.DeleteFactor(context.Background(), "factor-1")
	require.NoError(t, err)
}

func TestMFARepository_GetUnusedBackupCodesCount_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 8
			return nil
		}))

	count, err := repo.GetUnusedBackupCodesCount(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, 8, count)
}

func TestMFARepository_DeleteAllBackupCodes_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMFARepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 10"), nil)

	err := repo.DeleteAllBackupCodes(context.Background(), "user-1")
	require.NoError(t, err)
}

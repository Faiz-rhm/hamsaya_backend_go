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

func newAdminRepo(pool *testutil.MockPool) repositories.AdminRepository {
	return repositories.NewAdminRepository(testutil.NewTestDB(pool))
}

func TestAdminRepository_GetDashboardStats_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			for i := range dest {
				*dest[i].(*int64) = int64(i + 1)
			}
			return nil
		}))

	stats, err := repo.GetDashboardStats(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalUsers)
}

func TestAdminRepository_GetDashboardStats_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("db error")))

	_, err := repo.GetDashboardStats(context.Background())
	require.Error(t, err)
}

func TestAdminRepository_SuspendUser_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.SuspendUser(context.Background(), "user-1", time.Now().Add(24*time.Hour))
	require.NoError(t, err)
}

func TestAdminRepository_UnsuspendUser_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UnsuspendUser(context.Background(), "user-1")
	require.NoError(t, err)
}

func TestAdminRepository_UpdateUserRole_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateUserRole(context.Background(), "user-1", models.RoleAdmin)
	require.NoError(t, err)
}

func TestAdminRepository_DeleteUser_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.DeleteUser(context.Background(), "user-1")
	require.NoError(t, err)
}

func TestAdminRepository_IsIPBanned_True(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		}))

	banned, err := repo.IsIPBanned(context.Background(), "1.2.3.4")
	require.NoError(t, err)
	assert.True(t, banned)
}

func TestAdminRepository_IsIPBanned_False(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = false
			return nil
		}))

	banned, err := repo.IsIPBanned(context.Background(), "10.0.0.1")
	require.NoError(t, err)
	assert.False(t, banned)
}

func TestAdminRepository_IsDeviceBanned_True(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		}))

	banned, err := repo.IsDeviceBanned(context.Background(), "device-abc")
	require.NoError(t, err)
	assert.True(t, banned)
}

func TestAdminRepository_IsDeviceBanned_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("db error")))

	_, err := repo.IsDeviceBanned(context.Background(), "device-xyz")
	require.Error(t, err)
}

func TestAdminRepository_UpdatePostStatus_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdatePostStatus(context.Background(), "post-1", "removed")
	require.NoError(t, err)
}

func TestAdminRepository_DeletePost_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.DeletePost(context.Background(), "post-1")
	require.NoError(t, err)
}

func TestAdminRepository_GetAllUserIDs_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(
			func(dest ...any) error {
				*dest[0].(*string) = "user-1"
				return nil
			},
			func(dest ...any) error {
				*dest[0].(*string) = "user-2"
				return nil
			},
		), nil)

	ids, err := repo.GetAllUserIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"user-1", "user-2"}, ids)
}

func TestAdminRepository_CreateAuditLog_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newAdminRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	req := &models.CreateAuditLogRequest{
		AdminID: "admin-1", Action: "delete_post", EntityType: "post", EntityID: "post-1",
	}
	err := repo.CreateAuditLog(context.Background(), req)
	require.NoError(t, err)
}

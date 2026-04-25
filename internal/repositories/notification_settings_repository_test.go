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

func newNotificationSettingsRepo(pool *testutil.MockPool) repositories.NotificationSettingsRepository {
	return repositories.NewNotificationSettingsRepository(testutil.NewTestDB(pool))
}

func TestNotificationSettingsRepository_GetByProfileID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "ns-1"
			*dest[1].(*string) = "profile-1"
			*dest[2].(*models.NotificationCategory) = models.NotificationCategory("likes")
			*dest[3].(*bool) = true
			*dest[4].(*time.Time) = now
			*dest[5].(*time.Time) = now
			return nil
		}), nil)

	settings, err := repo.GetByProfileID(context.Background(), "profile-1")
	require.NoError(t, err)
	assert.Len(t, settings, 1)
	assert.Equal(t, "ns-1", settings[0].ID)
	assert.True(t, settings[0].PushPref)
}

func TestNotificationSettingsRepository_GetByProfileID_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, err := repo.GetByProfileID(context.Background(), "profile-1")
	require.Error(t, err)
}

func TestNotificationSettingsRepository_GetByProfileID_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	settings, err := repo.GetByProfileID(context.Background(), "profile-1")
	require.NoError(t, err)
	assert.Empty(t, settings)
}

func TestNotificationSettingsRepository_GetByProfileAndCategory_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "ns-1"
			*dest[1].(*string) = "profile-1"
			*dest[2].(*models.NotificationCategory) = models.NotificationCategory("likes")
			*dest[3].(*bool) = false
			*dest[4].(*time.Time) = now
			*dest[5].(*time.Time) = now
			return nil
		}))

	setting, err := repo.GetByProfileAndCategory(context.Background(), "profile-1", "likes")
	require.NoError(t, err)
	assert.Equal(t, "ns-1", setting.ID)
	assert.False(t, setting.PushPref)
}

func TestNotificationSettingsRepository_GetByProfileAndCategory_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	_, err := repo.GetByProfileAndCategory(context.Background(), "profile-1", "likes")
	require.Error(t, err)
}

func TestNotificationSettingsRepository_Upsert_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	setting := &models.NotificationSetting{
		ID: "ns-1", ProfileID: "profile-1",
		Category:  models.NotificationCategory("likes"),
		PushPref:  true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	err := repo.Upsert(context.Background(), setting)
	require.NoError(t, err)
}

func TestNotificationSettingsRepository_UpdateCategory_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateCategory(context.Background(), "profile-1", "likes", false)
	require.NoError(t, err)
}

func TestNotificationSettingsRepository_InitializeDefaults_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newNotificationSettingsRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 5"), nil)

	err := repo.InitializeDefaults(context.Background(), "profile-1")
	require.NoError(t, err)
}

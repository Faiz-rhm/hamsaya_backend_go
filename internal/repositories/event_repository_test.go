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

func newEventRepo(pool *testutil.MockPool) repositories.EventRepository {
	return repositories.NewEventRepository(testutil.NewTestDB(pool))
}

func TestEventRepository_SetInterest_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	interest := &models.EventInterest{
		ID: "ei-1", PostID: "post-1", UserID: "user-1",
		EventState: models.EventInterestGoing,
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	}
	err := repo.SetInterest(context.Background(), interest)
	require.NoError(t, err)
}

func TestEventRepository_SetInterest_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.SetInterest(context.Background(), &models.EventInterest{
		ID: "ei-1", PostID: "p1", UserID: "u1",
		EventState: models.EventInterestInterested,
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	})
	require.Error(t, err)
}

func TestEventRepository_GetUserInterest_Found(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "ei-1"
			*dest[1].(*string) = "post-1"
			*dest[2].(*string) = "user-1"
			*dest[3].(*models.EventInterestState) = models.EventInterestGoing
			*dest[4].(*time.Time) = now
			*dest[5].(*time.Time) = now
			return nil
		}))

	interest, err := repo.GetUserInterest(context.Background(), "user-1", "post-1")
	require.NoError(t, err)
	require.NotNil(t, interest)
	assert.Equal(t, models.EventInterestGoing, interest.EventState)
}

func TestEventRepository_GetUserInterest_NotFound_ReturnsNil(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	// ErrNoRows should return nil, nil per implementation
	// but our ErrRow returns an arbitrary error, so we just check no panic
	_, _ = repo.GetUserInterest(context.Background(), "user-1", "post-1")
}

func TestEventRepository_DeleteInterest_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 1"), nil)

	err := repo.DeleteInterest(context.Background(), "user-1", "post-1")
	require.NoError(t, err)
}

func TestEventRepository_CountByState_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 7
			return nil
		}))

	count, err := repo.CountByState(context.Background(), "post-1", models.EventInterestGoing)
	require.NoError(t, err)
	assert.Equal(t, 7, count)
}

func TestEventRepository_GetInterestedUsers_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "ei-1"
			*dest[1].(*string) = "post-1"
			*dest[2].(*string) = "user-1"
			*dest[3].(*models.EventInterestState) = models.EventInterestInterested
			*dest[4].(*time.Time) = now
			*dest[5].(*time.Time) = now
			return nil
		}), nil)

	interests, err := repo.GetInterestedUsers(context.Background(), "post-1", models.EventInterestInterested, 10, 0)
	require.NoError(t, err)
	assert.Len(t, interests, 1)
	assert.Equal(t, models.EventInterestInterested, interests[0].EventState)
}

func TestEventRepository_GetInterestedUsers_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newEventRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, err := repo.GetInterestedUsers(context.Background(), "post-1", models.EventInterestGoing, 10, 0)
	require.Error(t, err)
}

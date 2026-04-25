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

func newFeedbackRepo(pool *testutil.MockPool) repositories.FeedbackRepository {
	return repositories.NewFeedbackRepository(testutil.NewTestDB(pool))
}

func TestFeedbackRepository_Create_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	feedback := &models.Feedback{
		UserID: "user-1", Rating: models.FeedbackRatingGood,
		Type: models.FeedbackTypeGeneral, Message: "Love it!",
	}
	err := repo.Create(context.Background(), feedback)
	require.NoError(t, err)
	assert.NotEmpty(t, feedback.ID)
}

func TestFeedbackRepository_Create_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.Create(context.Background(), &models.Feedback{
		UserID: "u1", Rating: models.FeedbackRatingGood, Type: models.FeedbackTypeGeneral, Message: "ok",
	})
	require.Error(t, err)
}

func TestFeedbackRepository_GetUserFeedbackStatus_HasFeedback(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*time.Time) = now
			return nil
		}))

	hasFeedback, lastAt, err := repo.GetUserFeedbackStatus(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, hasFeedback)
	assert.NotNil(t, lastAt)
}

func TestFeedbackRepository_GetUserFeedbackStatus_NoFeedback(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows in result set")))

	hasFeedback, lastAt, err := repo.GetUserFeedbackStatus(context.Background(), "user-1")
	require.NoError(t, err)
	assert.False(t, hasFeedback)
	assert.Nil(t, lastAt)
}

func TestFeedbackRepository_GetByUserID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "fb-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*models.FeedbackRating) = models.FeedbackRatingGood
			*dest[3].(*models.FeedbackType) = models.FeedbackTypeGeneral
			*dest[4].(*string) = "Nice app"
			*dest[5].(**string) = nil
			*dest[6].(**string) = nil
			*dest[7].(*time.Time) = now
			return nil
		}), nil)

	feedbacks, err := repo.GetByUserID(context.Background(), "user-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, feedbacks, 1)
	assert.Equal(t, "fb-1", feedbacks[0].ID)
}

func TestFeedbackRepository_GetByUserID_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newFeedbackRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	feedbacks, err := repo.GetByUserID(context.Background(), "user-1", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, feedbacks)
}

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

func newPollRepo(pool *testutil.MockPool) repositories.PollRepository {
	return repositories.NewPollRepository(testutil.NewTestDB(pool))
}

func TestPollRepository_Create_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	poll := &models.Poll{ID: "poll-1", PostID: "post-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err := repo.Create(context.Background(), poll)
	require.NoError(t, err)
}

func TestPollRepository_Create_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.Create(context.Background(), &models.Poll{ID: "poll-1", PostID: "post-1"})
	require.Error(t, err)
}

func TestPollRepository_GetByID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "poll-1"
			*dest[1].(*string) = "post-1"
			*dest[2].(*time.Time) = now
			*dest[3].(*time.Time) = now
			*dest[4].(**time.Time) = nil
			return nil
		}))

	poll, err := repo.GetByID(context.Background(), "poll-1")
	require.NoError(t, err)
	assert.Equal(t, "poll-1", poll.ID)
	assert.Equal(t, "post-1", poll.PostID)
}

func TestPollRepository_GetByID_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	_, err := repo.GetByID(context.Background(), "not-exist")
	require.Error(t, err)
}

func TestPollRepository_Delete_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.Delete(context.Background(), "poll-1")
	require.NoError(t, err)
}

func TestPollRepository_CreateOption_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	opt := &models.PollOption{ID: "opt-1", PollID: "poll-1", Option: "Apple", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err := repo.CreateOption(context.Background(), opt)
	require.NoError(t, err)
}

func TestPollRepository_GetOptionsByPollID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	now := time.Now()
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "opt-1"
			*dest[1].(*string) = "poll-1"
			*dest[2].(*string) = "Apple"
			*dest[3].(*int) = 5
			*dest[4].(*time.Time) = now
			*dest[5].(*time.Time) = now
			return nil
		}), nil)

	opts, err := repo.GetOptionsByPollID(context.Background(), "poll-1")
	require.NoError(t, err)
	assert.Len(t, opts, 1)
	assert.Equal(t, "Apple", opts[0].Option)
	assert.Equal(t, 5, opts[0].VoteCount)
}

func TestPollRepository_GetOptionsByPollID_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("query error"))

	_, err := repo.GetOptionsByPollID(context.Background(), "poll-1")
	require.Error(t, err)
}

func TestPollRepository_VotePoll_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	vote := &models.UserPoll{ID: "vote-1", UserID: "user-1", PollID: "poll-1", PollOptionID: "opt-1", CreatedAt: time.Now()}
	err := repo.VotePoll(context.Background(), vote)
	require.NoError(t, err)
}

func TestPollRepository_GetUserVote_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	now := time.Now()
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "vote-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "poll-1"
			*dest[3].(*string) = "opt-1"
			*dest[4].(*time.Time) = now
			return nil
		}))

	vote, err := repo.GetUserVote(context.Background(), "user-1", "poll-1")
	require.NoError(t, err)
	assert.Equal(t, "opt-1", vote.PollOptionID)
}

func TestPollRepository_UpdateOptionVoteCount_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newPollRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateOptionVoteCount(context.Background(), "opt-1", 1)
	require.NoError(t, err)
}

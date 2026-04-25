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

func newHelpChatRepo(pool *testutil.MockPool) repositories.HelpChatRepository {
	return repositories.NewHelpChatRepository(testutil.NewTestDB(pool))
}

func TestHelpChatRepository_CreateMessage_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	msg := &models.HelpChatMessage{
		UserID:     "user-1",
		Content:    "Hello support",
		IsFromUser: true,
	}

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*string) = "msg-uuid-1"
			*dest[1].(*time.Time) = time.Now()
			return nil
		}))

	err := repo.CreateMessage(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, "msg-uuid-1", msg.ID)
}

func TestHelpChatRepository_CreateMessage_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("db error")))

	err := repo.CreateMessage(context.Background(), &models.HelpChatMessage{
		UserID: "user-1", Content: "hi", IsFromUser: true,
	})
	require.Error(t, err)
}

func TestHelpChatRepository_GetMessages_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	now := time.Now()
	appVer := "1.0.0"
	deviceInfo := "iPhone"

	// COUNT query
	pool.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return len(sql) > 0
	}), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 1
			return nil
		})).Once()

	// Rows query
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "msg-1"
			*dest[1].(*string) = "user-1"
			*dest[2].(*string) = "Hello support"
			*dest[3].(*bool) = true
			*dest[4].(**string) = &appVer
			*dest[5].(**string) = &deviceInfo
			*dest[6].(*time.Time) = now
			return nil
		}), nil)

	msgs, total, err := repo.GetMessages(context.Background(), "user-1", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "msg-1", msgs[0].ID)
}

func TestHelpChatRepository_GetMessages_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 0
			return nil
		}))
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("query error"))

	_, _, err := repo.GetMessages(context.Background(), "user-1", 50, 0)
	require.Error(t, err)
}

func TestHelpChatRepository_GetMessages_Empty(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 0
			return nil
		}))
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	msgs, total, err := repo.GetMessages(context.Background(), "user-1", 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, msgs)
}

func TestHelpChatRepository_GetAllUserThreads_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	now := time.Now()

	pool.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 1
			return nil
		}))
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewFuncRows(func(dest ...any) error {
			*dest[0].(*string) = "user-1"
			*dest[1].(*string) = "John Doe"
			*dest[2].(*string) = "john@example.com"
			*dest[3].(*string) = "Hello"
			*dest[4].(*bool) = true
			*dest[5].(*time.Time) = now
			return nil
		}), nil)

	threads, total, err := repo.GetAllUserThreads(context.Background(), 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, threads, 1)
	assert.Equal(t, "user-1", threads[0].UserID)
}

func TestHelpChatRepository_GetAllUserThreads_QueryError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 0
			return nil
		}))
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("db error"))

	_, _, err := repo.GetAllUserThreads(context.Background(), 50, 0)
	require.Error(t, err)
}

func TestHelpChatRepository_GetUserMessages_DelegatesToGetMessages(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newHelpChatRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int64) = 0
			return nil
		}))
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.EmptyRows(), nil)

	msgs, total, err := repo.GetUserMessages(context.Background(), "user-1", 50, 0)
	require.NoError(t, err)
	assert.Empty(t, msgs)
	assert.Equal(t, int64(0), total)
	_ = pgconn.CommandTag{}
}

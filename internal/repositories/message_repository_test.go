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

func newMessageRepo(pool *testutil.MockPool) repositories.MessageRepository {
	return repositories.NewMessageRepository(testutil.NewTestDB(pool))
}

func makeMessageScanFn(m *models.Message) func(dest ...any) error {
	return func(dest ...any) error {
		content := ""
		if m.Content != nil {
			content = *m.Content
		}
		contentPtr := &content
		*dest[0].(*string) = m.ID
		*dest[1].(*string) = m.ConversationID
		*dest[2].(*string) = m.SenderID
		*dest[3].(**string) = contentPtr
		*dest[4].(*models.MessageType) = m.MessageType
		*dest[5].(**string) = m.ProductID
		*dest[6].(**time.Time) = m.ReadAt
		*dest[7].(*time.Time) = m.CreatedAt
		*dest[8].(**time.Time) = m.DeletedAt
		return nil
	}
}

func TestMessageRepository_Create_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("INSERT 1"), nil)

	content := "Hello"
	msg := &models.Message{
		ID: "msg-1", ConversationID: "conv-1", SenderID: "user-1",
		Content: &content, MessageType: models.MessageTypeText, CreatedAt: time.Now(),
	}
	err := repo.Create(context.Background(), msg)
	require.NoError(t, err)
}

func TestMessageRepository_Create_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	content := "Hello"
	err := repo.Create(context.Background(), &models.Message{
		ID: "msg-1", ConversationID: "conv-1", SenderID: "user-1",
		Content: &content, MessageType: models.MessageTypeText, CreatedAt: time.Now(),
	})
	require.Error(t, err)
}

func TestMessageRepository_GetByID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	content := "Hello there"
	msg := &models.Message{
		ID: "msg-1", ConversationID: "conv-1", SenderID: "user-1",
		Content: &content, MessageType: models.MessageTypeText, CreatedAt: time.Now(),
	}
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(makeMessageScanFn(msg)))

	result, err := repo.GetByID(context.Background(), "msg-1")
	require.NoError(t, err)
	assert.Equal(t, "msg-1", result.ID)
	assert.Equal(t, models.MessageTypeText, result.MessageType)
}

func TestMessageRepository_GetByID_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	_, err := repo.GetByID(context.Background(), "not-exist")
	require.Error(t, err)
}

func TestMessageRepository_Delete_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.Delete(context.Background(), "msg-1")
	require.NoError(t, err)
}

func TestMessageRepository_MarkAsRead_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.MarkAsRead(context.Background(), "msg-1")
	require.NoError(t, err)
}

func TestMessageRepository_MarkConversationAsRead_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 5"), nil)

	err := repo.MarkConversationAsRead(context.Background(), "conv-1", "user-1")
	require.NoError(t, err)
}

func TestMessageRepository_GetUnreadCount_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 3
			return nil
		}))

	count, err := repo.GetUnreadCount(context.Background(), "conv-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestMessageRepository_GetLastMessage_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	content := "Last msg"
	msg := &models.Message{
		ID: "msg-last", ConversationID: "conv-1", SenderID: "user-1",
		Content: &content, MessageType: models.MessageTypeText, CreatedAt: time.Now(),
	}
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(makeMessageScanFn(msg)))

	result, err := repo.GetLastMessage(context.Background(), "conv-1")
	require.NoError(t, err)
	assert.Equal(t, "msg-last", result.ID)
}

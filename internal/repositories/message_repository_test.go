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
		*dest[6].(**string) = m.ReplyToMessageID
		*dest[7].(**time.Time) = m.ReadAt
		*dest[8].(*time.Time) = m.CreatedAt
		*dest[9].(**time.Time) = m.DeletedAt
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

	result, err := repo.GetLastMessage(context.Background(), "conv-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "msg-last", result.ID)
}

// --- DeleteForUser (delete-for-me) ---

func TestMessageRepository_DeleteForUser_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.DeleteForUser(context.Background(), "msg-1", "user-1")
	require.NoError(t, err)
}

func TestMessageRepository_DeleteForUser_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	// 0 rows affected means the row was either already deleted-for-everyone
	// or never existed — surface as an error so the service can return 404.
	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 0"), nil)

	err := repo.DeleteForUser(context.Background(), "msg-missing", "user-1")
	require.Error(t, err)
}

func TestMessageRepository_DeleteForUser_DBError(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag(""), errors.New("db error"))

	err := repo.DeleteForUser(context.Background(), "msg-1", "user-1")
	require.Error(t, err)
}

// --- Viewer-filter SQL assertions ---
// Each viewer-aware query must include the deleted_for_user_ids exclusion
// clause; a regression that drops it would silently show messages the user
// chose to hide. Match the exact column name so a rename is also caught.

func TestMessageRepository_List_IncludesViewerFilter(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	var capturedSQL string
	pool.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) {
			capturedSQL = args.String(1)
		}).
		Return(testutil.NewMockRows([][]any{}), nil)

	_, err := repo.List(context.Background(), &models.GetMessagesFilter{
		ConversationID: "conv-1",
		ViewerID:       "user-1",
		Limit:          10,
		Offset:         0,
	})
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "deleted_for_user_ids",
		"List must exclude rows the viewer has delete-for-me'd")
}

func TestMessageRepository_GetLastMessage_IncludesViewerFilter(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	var capturedSQL string
	content := "x"
	msg := &models.Message{
		ID: "msg-1", ConversationID: "conv-1", SenderID: "user-1",
		Content: &content, MessageType: models.MessageTypeText, CreatedAt: time.Now(),
	}
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) {
			capturedSQL = args.String(1)
		}).
		Return(testutil.NewMockRow(makeMessageScanFn(msg)))

	_, err := repo.GetLastMessage(context.Background(), "conv-1", "user-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "deleted_for_user_ids",
		"GetLastMessage preview must skip per-user-deleted rows for the viewer")
}

func TestMessageRepository_GetUnreadCount_IncludesViewerFilter(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newMessageRepo(pool)

	var capturedSQL string
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) {
			capturedSQL = args.String(1)
		}).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*int) = 0
			return nil
		}))

	_, err := repo.GetUnreadCount(context.Background(), "conv-1", "user-1")
	require.NoError(t, err)
	assert.Contains(t, capturedSQL, "deleted_for_user_ids",
		"GetUnreadCount must skip per-user-deleted rows so badge matches list")
}

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

func newConversationRepo(pool *testutil.MockPool) repositories.ConversationRepository {
	return repositories.NewConversationRepository(testutil.NewTestDB(pool))
}

func makeConversationScanFn(c *models.Conversation) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*string) = c.ID
		*dest[1].(*string) = c.Participant1ID
		*dest[2].(*string) = c.Participant2ID
		*dest[3].(**time.Time) = c.LastMessageAt
		*dest[4].(*time.Time) = c.CreatedAt
		return nil
	}
}

func TestConversationRepository_GetByID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	now := time.Now()
	conv := &models.Conversation{
		ID: "conv-1", Participant1ID: "user-a", Participant2ID: "user-b",
		LastMessageAt: nil, CreatedAt: now,
	}
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(makeConversationScanFn(conv)))

	result, err := repo.GetByID(context.Background(), "conv-1")
	require.NoError(t, err)
	assert.Equal(t, "conv-1", result.ID)
	assert.Equal(t, "user-a", result.Participant1ID)
}

func TestConversationRepository_GetByID_NotFound(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.ErrRow(errors.New("no rows")))

	_, err := repo.GetByID(context.Background(), "not-exist")
	require.Error(t, err)
}

func TestConversationRepository_GetByParticipants_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	now := time.Now()
	conv := &models.Conversation{ID: "conv-1", Participant1ID: "user-a", Participant2ID: "user-b", CreatedAt: now}
	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(makeConversationScanFn(conv)))

	result, err := repo.GetByParticipants(context.Background(), "user-a", "user-b")
	require.NoError(t, err)
	assert.Equal(t, "conv-1", result.ID)
}

func TestConversationRepository_UpdateLastMessageAt_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := repo.UpdateLastMessageAt(context.Background(), "conv-1")
	require.NoError(t, err)
}

func TestConversationRepository_Delete_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(pgconn.NewCommandTag("DELETE 1"), nil)

	err := repo.Delete(context.Background(), "conv-1")
	require.NoError(t, err)
}

func TestConversationRepository_IsParticipant_True(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		}))

	ok, err := repo.IsParticipant(context.Background(), "conv-1", "user-a")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConversationRepository_IsParticipant_False(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			*dest[0].(*bool) = false
			return nil
		}))

	ok, err := repo.IsParticipant(context.Background(), "conv-1", "user-x")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConversationRepository_GetOtherParticipantID_Success(t *testing.T) {
	pool := new(testutil.MockPool)
	repo := newConversationRepo(pool)

	pool.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(testutil.NewMockRow(func(dest ...any) error {
			other := "user-b"
			*dest[0].(**string) = &other
			return nil
		}))

	otherID, err := repo.GetOtherParticipantID(context.Background(), "conv-1", "user-a")
	require.NoError(t, err)
	assert.Equal(t, "user-b", otherID)
}

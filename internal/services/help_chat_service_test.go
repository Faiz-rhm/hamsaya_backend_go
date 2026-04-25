package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newHelpChatService(repo *mocks.MockHelpChatRepository) *HelpChatService {
	return NewHelpChatService(repo, zap.NewNop())
}

// --- SendUserMessage ---

func TestHelpChatService_SendUserMessage_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*models.HelpChatMessage")).
		Return(nil).
		Run(func(args mock.Arguments) {
			msg := args.Get(1).(*models.HelpChatMessage)
			msg.ID = "msg-1"
			msg.CreatedAt = time.Now()
		})

	svc := newHelpChatService(repo)
	req := &models.SendHelpMessageRequest{Content: "Hello support"}
	msg, err := svc.SendUserMessage(context.Background(), "user-1", req)

	require.NoError(t, err)
	assert.Equal(t, "user-1", msg.UserID)
	assert.Equal(t, "Hello support", msg.Content)
	assert.True(t, msg.IsFromUser)
	repo.AssertExpectations(t)
}

func TestHelpChatService_SendUserMessage_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.Anything).Return(errors.New("db error"))

	svc := newHelpChatService(repo)
	_, err := svc.SendUserMessage(context.Background(), "user-1", &models.SendHelpMessageRequest{Content: "hi"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to send message")
}

// --- GetUserMessages ---

func TestHelpChatService_GetUserMessages_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	msgs := []*models.HelpChatMessage{{ID: "m1", UserID: "user-1", Content: "hi"}}
	repo.On("GetMessages", mock.Anything, "user-1", 50, 0).Return(msgs, int64(1), nil)

	svc := newHelpChatService(repo)
	result, total, err := svc.GetUserMessages(context.Background(), "user-1", 1, 50)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
}

func TestHelpChatService_GetUserMessages_ClampsLimit(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	// limit=200 should be clamped to 50; page=1 → offset=0
	repo.On("GetMessages", mock.Anything, "user-1", 50, 0).Return([]*models.HelpChatMessage{}, int64(0), nil)

	svc := newHelpChatService(repo)
	_, _, err := svc.GetUserMessages(context.Background(), "user-1", 1, 200)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestHelpChatService_GetUserMessages_NegativeLimitDefaulted(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("GetMessages", mock.Anything, "user-1", 50, 0).Return([]*models.HelpChatMessage{}, int64(0), nil)

	svc := newHelpChatService(repo)
	_, _, err := svc.GetUserMessages(context.Background(), "user-1", 1, -5)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// --- AdminReply ---

func TestHelpChatService_AdminReply_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*models.HelpChatMessage")).
		Return(nil).
		Run(func(args mock.Arguments) {
			msg := args.Get(1).(*models.HelpChatMessage)
			msg.ID = "msg-admin-1"
		})

	svc := newHelpChatService(repo)
	req := &models.AdminReplyRequest{Content: "We will help you."}
	msg, err := svc.AdminReply(context.Background(), "admin-1", "user-1", req)

	require.NoError(t, err)
	assert.Equal(t, "user-1", msg.UserID)
	assert.False(t, msg.IsFromUser)
	assert.Equal(t, "We will help you.", msg.Content)
}

func TestHelpChatService_AdminReply_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.Anything).Return(errors.New("db error"))

	svc := newHelpChatService(repo)
	_, err := svc.AdminReply(context.Background(), "admin-1", "user-1", &models.AdminReplyRequest{Content: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to send reply")
}

// --- GetAllThreads ---

func TestHelpChatService_GetAllThreads_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	threads := []*models.HelpChatThread{{UserID: "user-1", Email: "a@b.com", LastMessage: "hi"}}
	repo.On("GetAllUserThreads", mock.Anything, 50, 0).Return(threads, int64(1), nil)

	svc := newHelpChatService(repo)
	result, total, err := svc.GetAllThreads(context.Background(), 1, 50)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
}

func TestHelpChatService_GetAllThreads_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("GetAllUserThreads", mock.Anything, 50, 0).Return(nil, int64(0), errors.New("db error"))

	svc := newHelpChatService(repo)
	_, _, err := svc.GetAllThreads(context.Background(), 1, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to get threads")
}

// --- GetUserThread ---

func TestHelpChatService_GetUserThread_DelegatesToGetUserMessages(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	msgs := []*models.HelpChatMessage{{ID: "m1", UserID: "user-1"}}
	repo.On("GetMessages", mock.Anything, "user-1", 50, 0).Return(msgs, int64(1), nil)

	svc := newHelpChatService(repo)
	result, total, err := svc.GetUserThread(context.Background(), "user-1", 1, 50)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
}

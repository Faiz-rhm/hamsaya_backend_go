package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestChatService(convRepo *mocks.MockConversationRepository, msgRepo *mocks.MockMessageRepository, userRepo *mocks.MockUserRepository) *ChatService {
	return NewChatService(convRepo, msgRepo, userRepo, nil, zap.NewNop())
}

func newTestConversation(id string) *models.Conversation {
	return &models.Conversation{ID: id, CreatedAt: time.Now()}
}

func newTestMessage(id, convID, senderID string) *models.Message {
	content := "hello"
	return &models.Message{
		ID:             id,
		ConversationID: convID,
		SenderID:       senderID,
		Content:        &content,
		MessageType:    models.MessageTypeText,
		CreatedAt:      time.Now(),
	}
}

func TestChatService_SendMessage(t *testing.T) {
	t.Run("empty text content rejected", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		empty := ""
		resp, err := svc.SendMessage(context.Background(), "sender-1", &models.SendMessageRequest{
			RecipientID: "recv-1",
			MessageType: models.MessageTypeText,
			Content:     &empty,
		})

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "content")
		assert.Nil(t, resp)
	})

	t.Run("nil text content rejected", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		resp, err := svc.SendMessage(context.Background(), "sender-1", &models.SendMessageRequest{
			RecipientID: "recv-1",
			MessageType: models.MessageTypeText,
			Content:     nil,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("get or create conversation fails", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("GetOrCreate", mock.Anything, "sender-1", "recv-1").
			Return(nil, errors.New("db error"))

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		content := "hi"
		resp, err := svc.SendMessage(context.Background(), "sender-1", &models.SendMessageRequest{
			RecipientID: "recv-1",
			MessageType: models.MessageTypeText,
			Content:     &content,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("message create fails", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		conv := newTestConversation("conv-1")
		convRepo.On("GetOrCreate", mock.Anything, "sender-1", "recv-1").Return(conv, nil)
		msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Message")).Return(errors.New("db error"))

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		content := "hi"
		resp, err := svc.SendMessage(context.Background(), "sender-1", &models.SendMessageRequest{
			RecipientID: "recv-1",
			MessageType: models.MessageTypeText,
			Content:     &content,
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		conv := newTestConversation("conv-1")
		convRepo.On("GetOrCreate", mock.Anything, "sender-1", "recv-1").Return(conv, nil)
		msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Message")).Return(nil)
		convRepo.On("UpdateLastMessageAt", mock.Anything, "conv-1").Return(nil)
		// enrichMessage calls GetProfileByUserID
		userRepo.On("GetProfileByUserID", mock.Anything, "sender-1").
			Return(&models.Profile{ID: "sender-1"}, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		content := "hello"
		resp, err := svc.SendMessage(context.Background(), "sender-1", &models.SendMessageRequest{
			RecipientID: "recv-1",
			MessageType: models.MessageTypeText,
			Content:     &content,
		})

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "conv-1", resp.ConversationID)
		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})
}

func TestChatService_GetConversations(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetConversationsFilter")).
			Return(nil, errors.New("db error"))

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		result, err := svc.GetConversations(context.Background(), "user-1", 10, 0)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("success empty", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetConversationsFilter")).
			Return([]*models.Conversation{}, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		result, err := svc.GetConversations(context.Background(), "user-1", 10, 0)

		require.NoError(t, err)
		_ = result // nil for empty slice is fine
	})

	t.Run("success with conversation", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		conv := newTestConversation("conv-1")
		convRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetConversationsFilter")).
			Return([]*models.Conversation{conv}, nil)
		convRepo.On("GetOtherParticipantID", mock.Anything, "conv-1", "user-1").Return("other-1", nil)
		userRepo.On("GetProfileByUserID", mock.Anything, "other-1").Return(&models.Profile{ID: "other-1"}, nil)
		msgRepo.On("GetLastMessage", mock.Anything, "conv-1").Return(nil, nil)
		msgRepo.On("GetUnreadCount", mock.Anything, "conv-1", "user-1").Return(0, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		result, err := svc.GetConversations(context.Background(), "user-1", 10, 0)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		convRepo.AssertExpectations(t)
	})
}

func TestChatService_GetMessages(t *testing.T) {
	t.Run("not participant", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("IsParticipant", mock.Anything, "conv-1", "user-1").Return(false, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		result, err := svc.GetMessages(context.Background(), "user-1", "conv-1", 10, 0)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("participant check error", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("IsParticipant", mock.Anything, "conv-1", "user-1").Return(false, errors.New("db error"))

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		_, err := svc.GetMessages(context.Background(), "user-1", "conv-1", 10, 0)

		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("IsParticipant", mock.Anything, "conv-1", "user-1").Return(true, nil)
		msg := newTestMessage("msg-1", "conv-1", "user-1")
		msgRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetMessagesFilter")).
			Return([]*models.Message{msg}, nil)
		userRepo.On("GetProfileByUserID", mock.Anything, "user-1").
			Return(&models.Profile{ID: "user-1"}, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		result, err := svc.GetMessages(context.Background(), "user-1", "conv-1", 10, 0)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})
}

func TestChatService_MarkConversationAsRead(t *testing.T) {
	t.Run("not participant", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("IsParticipant", mock.Anything, "conv-1", "user-1").Return(false, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		err := svc.MarkConversationAsRead(context.Background(), "user-1", "conv-1")

		require.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		convRepo.On("IsParticipant", mock.Anything, "conv-1", "user-1").Return(true, nil)
		msgRepo.On("MarkConversationAsRead", mock.Anything, "conv-1", "user-1").Return(nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		err := svc.MarkConversationAsRead(context.Background(), "user-1", "conv-1")

		require.NoError(t, err)
		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})
}

func TestChatService_DeleteMessage(t *testing.T) {
	t.Run("message not found", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		msgRepo.On("GetByID", mock.Anything, "msg-bad").Return(nil, errors.New("not found"))

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		err := svc.DeleteMessage(context.Background(), "user-1", "msg-bad")

		require.Error(t, err)
	})

	t.Run("not sender", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		msg := newTestMessage("msg-1", "conv-1", "other-user")
		msgRepo.On("GetByID", mock.Anything, "msg-1").Return(msg, nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		err := svc.DeleteMessage(context.Background(), "user-1", "msg-1")

		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "own messages")
	})

	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := new(mocks.MockUserRepository)

		msg := newTestMessage("msg-1", "conv-1", "user-1")
		msgRepo.On("GetByID", mock.Anything, "msg-1").Return(msg, nil)
		msgRepo.On("Delete", mock.Anything, "msg-1").Return(nil)

		svc := newTestChatService(convRepo, msgRepo, userRepo)
		err := svc.DeleteMessage(context.Background(), "user-1", "msg-1")

		require.NoError(t, err)
		msgRepo.AssertExpectations(t)
	})
}

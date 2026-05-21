package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	chatTestUserID        = "chat-user-001"
	chatTestConvID        = "chat-conv-001"
	chatTestMessageID     = "chat-msg-001"
	chatTestRecipientID   = "22222222-2222-2222-2222-222222222222"
)

func newChatRouter(
	t *testing.T,
	convRepo *mocks.MockConversationRepository,
	msgRepo *mocks.MockMessageRepository,
	userRepo ...*mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	var ur *mocks.MockUserRepository
	if len(userRepo) > 0 {
		ur = userRepo[0]
	} else {
		ur = &mocks.MockUserRepository{}
	}
	svc := services.NewChatService(convRepo, msgRepo, ur, nil, nil, nil, nil, zap.NewNop())
	cfg := &config.Config{CORS: config.CORSConfig{AllowedOrigins: []string{"*"}}}
	h := NewChatHandler(svc, nil, testutil.CreateTestValidator(), zap.NewNop(), cfg)

	authed := authContextMiddleware(chatTestUserID, "chat-sess-001")
	r := gin.New()
	r.POST("/api/v1/chat/messages", authed, h.SendMessage)
	r.GET("/api/v1/chat/conversations", authed, h.GetConversations)
	r.GET("/api/v1/chat/conversations/:conversation_id/messages", authed, h.GetMessages)
	r.POST("/api/v1/chat/conversations/:conversation_id/read", authed, h.MarkConversationAsRead)
	r.DELETE("/api/v1/chat/messages/:message_id", authed, h.DeleteMessage)
	r.POST("/api/v1/chat/messages/:message_id/delete-for-me", authed, h.DeleteMessageForMe)
	r.POST("/api/v1/noauth/chat/messages/:message_id/delete-for-me", h.DeleteMessageForMe)

	r.POST("/api/v1/noauth/chat/messages", h.SendMessage)
	return r
}

// --- SendMessage ---

func TestChatHandler_SendMessage(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newChatRouter(t, &mocks.MockConversationRepository{}, &mocks.MockMessageRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/chat/messages",
			strings.NewReader(`{"recipient_id":"`+chatTestRecipientID+`","message_type":"TEXT"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newChatRouter(t, &mocks.MockConversationRepository{}, &mocks.MockMessageRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/messages",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		r := newChatRouter(t, &mocks.MockConversationRepository{}, &mocks.MockMessageRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/messages",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		userRepo := &mocks.MockUserRepository{}
		conv := &models.Conversation{ID: chatTestConvID}
		convRepo.On("GetOrCreate", mock.Anything, chatTestUserID, chatTestRecipientID, mock.Anything).Return(conv, nil)
		msgRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Message")).Return(nil)
		convRepo.On("UpdateLastMessageAt", mock.Anything, chatTestConvID).Return(nil)
		userRepo.On("GetProfileByUserID", mock.Anything, mock.Anything).Return(&models.Profile{}, nil).Maybe()

		r := newChatRouter(t, convRepo, msgRepo, userRepo)
		body := `{"recipient_id":"` + chatTestRecipientID + `","message_type":"TEXT","content":"hello"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

// --- GetConversations ---

func TestChatHandler_GetConversations(t *testing.T) {
	t.Run("success empty", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		convRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetConversationsFilter")).
			Return([]*models.Conversation{}, nil)
		r := newChatRouter(t, convRepo, &mocks.MockMessageRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/conversations", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
		convRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		convRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetConversationsFilter")).
			Return(nil, fmt.Errorf("db error"))
		r := newChatRouter(t, convRepo, &mocks.MockMessageRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/conversations", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// --- GetMessages ---

func TestChatHandler_GetMessages(t *testing.T) {
	t.Run("not participant", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		convRepo.On("IsParticipant", mock.Anything, chatTestConvID, chatTestUserID).Return(false, nil)
		r := newChatRouter(t, convRepo, &mocks.MockMessageRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+chatTestConvID+"/messages", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("success empty", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		convRepo.On("IsParticipant", mock.Anything, chatTestConvID, chatTestUserID).Return(true, nil)
		msgRepo.On("List", mock.Anything, mock.AnythingOfType("*models.GetMessagesFilter")).
			Return([]*models.Message{}, nil)
		r := newChatRouter(t, convRepo, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+chatTestConvID+"/messages", nil)
		r.ServeHTTP(w, req)

		assert.Less(t, w.Code, 500)
	})
}

// --- MarkConversationAsRead ---

func TestChatHandler_MarkConversationAsRead(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		convRepo := &mocks.MockConversationRepository{}
		msgRepo := &mocks.MockMessageRepository{}
		convRepo.On("IsParticipant", mock.Anything, chatTestConvID, chatTestUserID).Return(true, nil)
		msgRepo.On("MarkConversationAsRead", mock.Anything, chatTestConvID, chatTestUserID).Return(nil)
		r := newChatRouter(t, convRepo, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+chatTestConvID+"/read", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		msgRepo.AssertExpectations(t)
	})
}

// --- DeleteMessage ---

func TestChatHandler_DeleteMessage(t *testing.T) {
	t.Run("message not found", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(nil, fmt.Errorf("not found"))
		r := newChatRouter(t, &mocks.MockConversationRepository{}, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/messages/"+chatTestMessageID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("not message owner", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		msg := &models.Message{ID: chatTestMessageID, SenderID: "other-user"}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(msg, nil)
		r := newChatRouter(t, &mocks.MockConversationRepository{}, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/messages/"+chatTestMessageID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		msg := &models.Message{ID: chatTestMessageID, SenderID: chatTestUserID}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(msg, nil)
		msgRepo.On("Delete", mock.Anything, chatTestMessageID).Return(nil)
		r := newChatRouter(t, &mocks.MockConversationRepository{}, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/messages/"+chatTestMessageID, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		msgRepo.AssertExpectations(t)
	})
}

// --- DeleteMessageForMe ---

func TestChatHandler_DeleteMessageForMe(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		r := newChatRouter(t, &mocks.MockConversationRepository{}, &mocks.MockMessageRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(
			http.MethodPost,
			"/api/v1/noauth/chat/messages/"+chatTestMessageID+"/delete-for-me",
			nil,
		)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("message not found", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(nil, fmt.Errorf("not found"))
		r := newChatRouter(t, &mocks.MockConversationRepository{}, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(
			http.MethodPost,
			"/api/v1/chat/messages/"+chatTestMessageID+"/delete-for-me",
			nil,
		)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("non-participant rejected", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		convRepo := &mocks.MockConversationRepository{}
		msg := &models.Message{ID: chatTestMessageID, SenderID: "other-user", ConversationID: chatTestConvID}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(msg, nil)
		convRepo.On("IsParticipant", mock.Anything, chatTestConvID, chatTestUserID).Return(false, nil)
		r := newChatRouter(t, convRepo, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(
			http.MethodPost,
			"/api/v1/chat/messages/"+chatTestMessageID+"/delete-for-me",
			nil,
		)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("success — recipient hides sender's message", func(t *testing.T) {
		msgRepo := &mocks.MockMessageRepository{}
		convRepo := &mocks.MockConversationRepository{}
		msg := &models.Message{ID: chatTestMessageID, SenderID: "other-user", ConversationID: chatTestConvID}
		msgRepo.On("GetByID", mock.Anything, chatTestMessageID).Return(msg, nil)
		convRepo.On("IsParticipant", mock.Anything, chatTestConvID, chatTestUserID).Return(true, nil)
		msgRepo.On("DeleteForUser", mock.Anything, chatTestMessageID, chatTestUserID).Return(nil)
		r := newChatRouter(t, convRepo, msgRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(
			http.MethodPost,
			"/api/v1/chat/messages/"+chatTestMessageID+"/delete-for-me",
			nil,
		)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		msgRepo.AssertExpectations(t)
		convRepo.AssertExpectations(t)
	})
}

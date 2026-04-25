package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const helpChatTestUserID = "help-user-001"
const helpChatTestAdminID = "help-admin-001"

func newHelpChatRouter(t *testing.T, repo *mocks.MockHelpChatRepository) *gin.Engine {
	t.Helper()
	svc := services.NewHelpChatService(repo, zap.NewNop())
	h := NewHelpChatHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	userAuth := authContextMiddleware(helpChatTestUserID, "sess-001")
	adminAuth := authContextMiddleware(helpChatTestAdminID, "sess-admin-001")

	r := gin.New()
	r.POST("/help-chat/messages", userAuth, h.SendMessage)
	r.GET("/help-chat/messages", userAuth, h.GetMessages)
	r.GET("/admin/help-chat", adminAuth, h.AdminGetThreads)
	r.GET("/admin/help-chat/:user_id", adminAuth, h.AdminGetUserThread)
	r.POST("/admin/help-chat/:user_id/reply", adminAuth, h.AdminReply)
	return r
}

// --- SendMessage ---

func TestHelpChatHandler_SendMessage_InvalidJSON(t *testing.T) {
	r := newHelpChatRouter(t, &mocks.MockHelpChatRepository{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/help-chat/messages", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHelpChatHandler_SendMessage_MissingContent(t *testing.T) {
	r := newHelpChatRouter(t, &mocks.MockHelpChatRepository{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/help-chat/messages", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHelpChatHandler_SendMessage_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*models.HelpChatMessage")).
		Return(errors.New("db error"))

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/help-chat/messages",
		strings.NewReader(`{"content":"need help please"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHelpChatHandler_SendMessage_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*models.HelpChatMessage")).
		Return(nil).
		Run(func(args mock.Arguments) {
			msg := args.Get(1).(*models.HelpChatMessage)
			msg.ID = "msg-1"
			msg.CreatedAt = time.Now()
		})

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/help-chat/messages",
		strings.NewReader(`{"content":"need help please"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

// --- GetMessages ---

func TestHelpChatHandler_GetMessages_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	msgs := []*models.HelpChatMessage{{ID: "m1", UserID: helpChatTestUserID, Content: "hello"}}
	repo.On("GetMessages", mock.Anything, helpChatTestUserID, 50, 0).Return(msgs, int64(1), nil)

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/help-chat/messages?page=1&limit=50", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHelpChatHandler_GetMessages_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("GetMessages", mock.Anything, helpChatTestUserID, mock.Anything, mock.Anything).
		Return(nil, int64(0), utils.NewInternalError("Failed to get messages", errors.New("db error")))

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/help-chat/messages", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- AdminGetThreads ---

func TestHelpChatHandler_AdminGetThreads_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	threads := []*models.HelpChatThread{{UserID: "user-1", Email: "a@b.com", LastMessage: "hi"}}
	repo.On("GetAllUserThreads", mock.Anything, 50, 0).Return(threads, int64(1), nil)

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/help-chat", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHelpChatHandler_AdminGetThreads_RepoError(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("GetAllUserThreads", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, int64(0), utils.NewInternalError("Failed to get threads", errors.New("db error")))

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/help-chat", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- AdminGetUserThread ---

func TestHelpChatHandler_AdminGetUserThread_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	msgs := []*models.HelpChatMessage{{ID: "m1", UserID: "target-user"}}
	repo.On("GetMessages", mock.Anything, "target-user", 50, 0).Return(msgs, int64(1), nil)

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/help-chat/target-user", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- AdminReply ---

func TestHelpChatHandler_AdminReply_InvalidJSON(t *testing.T) {
	r := newHelpChatRouter(t, &mocks.MockHelpChatRepository{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/admin/help-chat/user-1/reply", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHelpChatHandler_AdminReply_Success(t *testing.T) {
	repo := &mocks.MockHelpChatRepository{}
	repo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*models.HelpChatMessage")).
		Return(nil).
		Run(func(args mock.Arguments) {
			msg := args.Get(1).(*models.HelpChatMessage)
			msg.ID = "reply-1"
		})

	r := newHelpChatRouter(t, repo)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/admin/help-chat/user-1/reply",
		strings.NewReader(`{"content":"Support team reply"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

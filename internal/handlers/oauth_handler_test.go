package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newOAuthRouter(t *testing.T) *gin.Engine {
	t.Helper()
	cfg := authTestConfig()
	// OAuthService verifies external tokens; only input-validation tests run here.
	oauthSvc := services.NewOAuthService(&config.Config{}, &mocks.MockUserRepository{}, zap.NewNop())

	tokenStorage, _ := newAuthTestTokenStorage(t)
	userRepo := &mocks.MockUserRepository{}
	passwordSvc := services.NewPasswordService()
	jwtSvc := services.NewJWTService(&cfg.JWT)
	authSvc := services.NewAuthService(
		userRepo,
		nil,
		passwordSvc,
		jwtSvc,
		nil,
		tokenStorage,
		nil,
		cfg,
		zap.NewNop(),
	)

	h := NewOAuthHandler(authSvc, oauthSvc, testutil.CreateTestValidator(), zap.NewNop())
	r := gin.New()
	r.POST("/api/v1/auth/oauth/google", h.GoogleOAuth)
	r.POST("/api/v1/auth/oauth/facebook", h.FacebookOAuth)
	r.POST("/api/v1/auth/oauth/apple", h.AppleOAuth)
	return r
}

// --- GoogleOAuth ---

func TestOAuthHandler_GoogleOAuth(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing id_token", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- FacebookOAuth ---

func TestOAuthHandler_FacebookOAuth(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/facebook",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing access_token", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/facebook",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- AppleOAuth ---

func TestOAuthHandler_AppleOAuth(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/apple",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing id_token", func(t *testing.T) {
		r := newOAuthRouter(t)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/oauth/apple",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// authTestPasswordHash is a known-good bcrypt hash for the string "password" (cost=12).
// Source: auth_service_test.go testPasswordHash constant (verified by service tests).
const authTestPasswordHash = "$2a$12$SK7HMTw9slXUVmPZtdMa6evdMIN5CBUFvQfwOBbLgcb.Tt8Bi9UpK"


func authTestConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:               "test-secret-key-at-least-32-characters-long-for-security",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 7 * 24 * time.Hour,
		},
	}
}

func newAuthTestTokenStorage(t *testing.T) (*services.TokenStorageService, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return services.NewTokenStorageService(rdb, zap.NewNop()), mr
}

func buildAuthHandler(t *testing.T, userRepo *mocks.MockUserRepository) *AuthHandler {
	t.Helper()
	cfg := authTestConfig()
	tokenStorage, _ := newAuthTestTokenStorage(t)
	jwtSvc := services.NewJWTService(&cfg.JWT)
	passwordSvc := services.NewPasswordService()
	emailSvc := services.NewEmailService(&config.EmailConfig{}, zap.NewNop())
	authSvc := services.NewAuthService(userRepo, nil, passwordSvc, jwtSvc, emailSvc, tokenStorage, nil, cfg, zap.NewNop())
	return NewAuthHandler(authSvc, testutil.CreateTestValidator(), zap.NewNop())
}

// authContextMiddleware injects user_id and session_id into the gin context,
// simulating a successful auth middleware pass.
func authContextMiddleware(userID, sessionID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("session_id", sessionID)
		c.Next()
	}
}

const (
	authTestUserID    = "user-auth-test-001"
	authTestSessionID = "session-auth-test-001"
)

func newAuthTestRouter(t *testing.T, userRepo *mocks.MockUserRepository) *gin.Engine {
	t.Helper()
	h := buildAuthHandler(t, userRepo)
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1, authContextMiddleware(authTestUserID, authTestSessionID))
	return r
}

func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

func assertResponse(t *testing.T, w *httptest.ResponseRecorder, wantCode int, wantSuccess bool) map[string]interface{} {
	t.Helper()
	assert.Equal(t, wantCode, w.Code)
	body := parseBody(t, w)
	assert.Equal(t, wantSuccess, body["success"])
	return body
}

// --- Register ---

func TestAuthHandler_Register(t *testing.T) {
	validBody := map[string]interface{}{
		"email": "new@example.com", "password": "Password1!",
		"first_name": "John", "last_name": "Doe",
		"latitude": 34.5, "longitude": 69.1,
	}

	tests := []struct {
		name        string
		body        interface{}
		setupMocks  func(*mocks.MockUserRepository)
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "not-json",
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing required fields",
			body:        map[string]interface{}{"email": "test@example.com"},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "invalid email format",
			body: map[string]interface{}{
				"email": "not-email", "password": "Password1!",
				"first_name": "John", "last_name": "Doe",
				"latitude": 34.5, "longitude": 69.1,
			},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "password too short",
			body: map[string]interface{}{
				"email": "new@example.com", "password": "short",
				"first_name": "John", "last_name": "Doe",
				"latitude": 34.5, "longitude": 69.1,
			},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "email already exists",
			body: map[string]interface{}{
				"email": "exists@example.com", "password": "Password1!",
				"first_name": "John", "last_name": "Doe",
				"latitude": 34.5, "longitude": 69.1,
			},
			setupMocks: func(r *mocks.MockUserRepository) {
				r.On("GetByEmail", mock.Anything, "exists@example.com").
					Return(testutil.CreateTestUser("user-1", "exists@example.com"), nil)
			},
			wantCode:    http.StatusConflict,
			wantSuccess: false,
		},
		{
			name: "email previously deleted — blocked",
			body: map[string]interface{}{
				"email": "deleted@example.com", "password": "Password1!",
				"first_name": "John", "last_name": "Doe",
				"latitude": 34.5, "longitude": 69.1,
			},
			setupMocks: func(r *mocks.MockUserRepository) {
				r.On("GetByEmail", mock.Anything, "deleted@example.com").
					Return(nil, fmt.Errorf("not found"))
				r.On("GetByEmailIncludingDeleted", mock.Anything, "deleted@example.com").
					Return(testutil.CreateTestUser("user-deleted", "deleted@example.com"), nil)
			},
			wantCode:    http.StatusConflict,
			wantSuccess: false,
		},
		{
			name: "success",
			body: validBody,
			setupMocks: func(r *mocks.MockUserRepository) {
				r.On("GetByEmail", mock.Anything, "new@example.com").
					Return(nil, fmt.Errorf("not found"))
				r.On("GetByEmailIncludingDeleted", mock.Anything, "new@example.com").
					Return(nil, fmt.Errorf("not found"))
				r.On("CreateUserWithProfile", mock.Anything,
					mock.AnythingOfType("*models.User"),
					mock.AnythingOfType("*models.Profile")).
					Return(nil)
				r.On("GetProfileByUserID", mock.Anything, mock.Anything).
					Return(testutil.CreateTestProfile("prof-new", "John", "Doe"), nil)
				r.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).
					Return(nil)
				r.On("UpdateLastLogin", mock.Anything, mock.Anything).Return(nil)
			},
			wantCode:    http.StatusCreated,
			wantSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- Login ---

func TestAuthHandler_Login(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		setupMocks  func(*mocks.MockUserRepository)
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing email",
			body:        map[string]interface{}{"password": "pass"},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing password",
			body:        map[string]interface{}{"email": "test@example.com"},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			// Locked + correct password → 403 (mobile shows suspended UI).
			// Wrong password against locked stays 401 (covered below).
			name: "locked account with correct password",
			body: map[string]interface{}{"email": "locked@example.com", "password": "password"},
			setupMocks: func(r *mocks.MockUserRepository) {
				lockTime := time.Now().Add(30 * time.Minute)
				user := testutil.CreateTestUser("user-1", "locked@example.com")
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				user.LockedUntil = &lockTime
				r.On("GetByEmail", mock.Anything, "locked@example.com").Return(user, nil)
			},
			wantCode:    http.StatusForbidden,
			wantSuccess: false,
		},
		{
			name: "wrong password increments attempt counter",
			body: map[string]interface{}{"email": "test@example.com", "password": "wrongpassword"},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				r.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				r.On("UpdateLoginAttempts", mock.Anything, "user-1", 1, (*time.Time)(nil)).Return(nil)
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name: "max attempts reached — account locked",
			body: map[string]interface{}{"email": "maxed@example.com", "password": "wrongpassword"},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-2", "maxed@example.com")
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				user.FailedLoginAttempts = services.MaxLoginAttempts - 1
				r.On("GetByEmail", mock.Anything, "maxed@example.com").Return(user, nil)
				r.On("UpdateLoginAttempts", mock.Anything, "user-2",
					services.MaxLoginAttempts, mock.AnythingOfType("*time.Time")).Return(nil)
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name: "success",
			body: map[string]interface{}{"email": "test@example.com", "password": "password"},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				// authTestPasswordHash is the known-good bcrypt hash for "password"
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				r.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
				r.On("GetProfileByUserID", mock.Anything, "user-1").
					Return(testutil.CreateTestProfile("prof-1", "Test", "User"), nil)
				r.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).Return(nil)
				r.On("UpdateLastLogin", mock.Anything, "user-1").Return(nil)
			},
			wantCode:    http.StatusOK,
			wantSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- RefreshToken ---

func TestAuthHandler_RefreshToken(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		setupMocks  func(*mocks.MockUserRepository)
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing refresh_token field",
			body:        map[string]interface{}{},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "token not found in DB — both lookups fail",
			body: map[string]interface{}{"refresh_token": "nonexistent-token"},
			setupMocks: func(r *mocks.MockUserRepository) {
				r.On("GetSessionByRefreshTokenHashAny", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("not found"))
				// Legacy fallback path also fails — covers the pre-hashing
				// migration code path.
				r.On("GetSessionByRefreshToken", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("not found"))
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name: "session revoked",
			body: map[string]interface{}{"refresh_token": "revoked-token"},
			setupMocks: func(r *mocks.MockUserRepository) {
				session := &models.UserSession{
					ID:        "session-1",
					UserID:    "user-1",
					Revoked:   true,
					ExpiresAt: time.Now().Add(time.Hour),
				}
				r.On("GetSessionByRefreshTokenHashAny", mock.Anything, mock.Anything).
					Return(session, nil)
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name: "session expired",
			body: map[string]interface{}{"refresh_token": "expired-token"},
			setupMocks: func(r *mocks.MockUserRepository) {
				session := &models.UserSession{
					ID:        "session-1",
					UserID:    "user-1",
					Revoked:   false,
					ExpiresAt: time.Now().Add(-time.Hour),
				}
				r.On("GetSessionByRefreshTokenHashAny", mock.Anything, mock.Anything).
					Return(session, nil)
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- Logout ---

func TestAuthHandler_Logout(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("RevokeSession", mock.Anything, authTestSessionID).Return(nil)
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		r.ServeHTTP(w, req)

		assertResponse(t, w, http.StatusOK, true)
		userRepo.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("RevokeSession", mock.Anything, authTestSessionID).
			Return(fmt.Errorf("db error"))
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})
}

// --- LogoutAll ---

func TestAuthHandler_LogoutAll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("RevokeAllUserSessions", mock.Anything, authTestUserID).Return(nil)
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
		r.ServeHTTP(w, req)

		assertResponse(t, w, http.StatusOK, true)
		userRepo.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("RevokeAllUserSessions", mock.Anything, authTestUserID).
			Return(fmt.Errorf("db failure"))
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})
}

// --- ForgotPassword ---

func TestAuthHandler_ForgotPassword(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		setupMocks func(*mocks.MockUserRepository)
		wantCode   int
	}{
		{
			name:       "invalid JSON",
			body:       "bad",
			setupMocks: func(r *mocks.MockUserRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "missing email",
			body:       map[string]interface{}{},
			setupMocks: func(r *mocks.MockUserRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "invalid email format",
			body:       map[string]interface{}{"email": "not-an-email"},
			setupMocks: func(r *mocks.MockUserRepository) {},
			wantCode:   http.StatusBadRequest,
		},
		{
			// Always returns 200 — avoids leaking whether email is registered
			name: "user not found — still 200",
			body: map[string]interface{}{"email": "nobody@example.com"},
			setupMocks: func(r *mocks.MockUserRepository) {
				r.On("GetByEmail", mock.Anything, "nobody@example.com").
					Return(nil, fmt.Errorf("not found"))
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- VerifyEmail ---

func TestAuthHandler_VerifyEmail(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		setupMocks  func(*mocks.MockUserRepository)
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing token",
			body:        map[string]interface{}{},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			// Token not in Redis → AppError 400 "Invalid or expired verification token"
			name:        "invalid token",
			body:        map[string]interface{}{"token": "totally-invalid-token"},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- ResetPassword ---

func TestAuthHandler_ResetPassword(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing fields",
			body:        map[string]interface{}{"token": "tok"},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			// Token not in Redis → AppError 400
			name:        "invalid token",
			body:        map[string]interface{}{"token": "invalid", "new_password": "Password1!"},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
		})
	}
}

// --- ChangePassword ---

func TestAuthHandler_ChangePassword(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		setupMocks  func(*mocks.MockUserRepository)
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing new_password",
			body:        map[string]interface{}{"current_password": "pass"},
			setupMocks:  func(r *mocks.MockUserRepository) {},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "wrong current password",
			body: map[string]interface{}{
				"current_password": "wrongpassword",
				"new_password":     "NewPassword1!",
			},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser(authTestUserID, "test@example.com")
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				r.On("GetByID", mock.Anything, authTestUserID).Return(user, nil)
			},
			wantCode:    http.StatusUnauthorized,
			wantSuccess: false,
		},
		{
			name: "new password same as current",
			body: map[string]interface{}{
				"current_password": "password",
				"new_password":     "password",
			},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser(authTestUserID, "test@example.com")
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				r.On("GetByID", mock.Anything, authTestUserID).Return(user, nil)
			},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name: "success",
			body: map[string]interface{}{
				"current_password": "password",
				"new_password":     "NewPassword1!",
			},
			setupMocks: func(r *mocks.MockUserRepository) {
				user := testutil.CreateTestUser(authTestUserID, "test@example.com")
				// authTestPasswordHash is the known-good hash for "password"
				user.PasswordHash = &[]string{authTestPasswordHash}[0]
				r.On("GetByID", mock.Anything, authTestUserID).Return(user, nil)
				r.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
				r.On("RevokeAllUserSessionsExcept", mock.Anything, authTestUserID, authTestSessionID).Return(nil)
				// GetProfileByUserID mock removed: confirmation email path
				// (the only consumer of the profile name) was dropped.
			},
			wantCode:    http.StatusOK,
			wantSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			tc.setupMocks(userRepo)
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/change-password", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
			userRepo.AssertExpectations(t)
		})
	}
}

// --- GetActiveSessions ---

func TestAuthHandler_GetActiveSessions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		sessions := []*models.UserSession{
			{ID: "sess-1", UserID: authTestUserID, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
			{ID: "sess-2", UserID: authTestUserID, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
		}
		userRepo.On("GetActiveSessions", mock.Anything, authTestUserID).Return(sessions, nil)
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
		r.ServeHTTP(w, req)

		body := assertResponse(t, w, http.StatusOK, true)
		data := body["data"].([]interface{})
		assert.Len(t, data, 2)
		userRepo.AssertExpectations(t)
	})

	t.Run("empty sessions list", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetActiveSessions", mock.Anything, authTestUserID).
			Return([]*models.UserSession{}, nil)
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
		r.ServeHTTP(w, req)

		assertResponse(t, w, http.StatusOK, true)
		userRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetActiveSessions", mock.Anything, authTestUserID).
			Return(nil, fmt.Errorf("db connection lost"))
		r := newAuthTestRouter(t, userRepo)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		userRepo.AssertExpectations(t)
	})
}

// --- VerifyResetCode ---

func TestAuthHandler_VerifyResetCode(t *testing.T) {
	tests := []struct {
		name        string
		body        interface{}
		wantCode    int
		wantSuccess bool
	}{
		{
			name:        "invalid JSON",
			body:        "bad",
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			name:        "missing fields",
			body:        map[string]interface{}{},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
		{
			// Token not in Redis → AppError 400
			name:        "invalid code",
			body:        map[string]interface{}{"email": "user@example.com", "token": "123456"},
			wantCode:    http.StatusBadRequest,
			wantSuccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := &mocks.MockUserRepository{}
			r := newAuthTestRouter(t, userRepo)

			var buf *bytes.Buffer
			if s, ok := tc.body.(string); ok {
				buf = bytes.NewBufferString(s)
			} else {
				buf = jsonBody(t, tc.body)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/verify-reset-code", buf)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assertResponse(t, w, tc.wantCode, tc.wantSuccess)
		})
	}
}

// --- handleError routing ---

func TestAuthHandler_HandleError_StatusMapping(t *testing.T) {
	// Verify that AppError codes are forwarded correctly by triggering
	// a conflict through Register with an existing email.
	userRepo := &mocks.MockUserRepository{}
	userRepo.On("GetByEmail", mock.Anything, "conflict@example.com").
		Return(testutil.CreateTestUser("u1", "conflict@example.com"), nil)
	r := newAuthTestRouter(t, userRepo)

	body := jsonBody(t, map[string]interface{}{
		"email": "conflict@example.com", "password": "Password1!",
		"first_name": "Alice", "last_name": "Baker",
		"latitude": 34.5, "longitude": 69.1,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- VerifyMFAWithBackupCode ---

func newAuthRouterWithMFA(t *testing.T, userRepo *mocks.MockUserRepository, mfaRepo *mocks.MockMFARepository, mr *miniredis.Miniredis) *gin.Engine {
	t.Helper()
	cfg := authTestConfig()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tokenStorage := services.NewTokenStorageService(rdb, zap.NewNop())
	jwtSvc := services.NewJWTService(&cfg.JWT)
	passwordSvc := services.NewPasswordService()
	emailSvc := services.NewEmailService(&config.EmailConfig{}, zap.NewNop())
	mfaSvc := services.NewMFAService(mfaRepo, userRepo, passwordSvc, zap.NewNop())
	authSvc := services.NewAuthService(userRepo, nil, passwordSvc, jwtSvc, emailSvc, tokenStorage, mfaSvc, cfg, zap.NewNop())
	h := NewAuthHandler(authSvc, testutil.CreateTestValidator(), zap.NewNop())
	r := gin.New()
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)
	return r
}

func TestAuthHandler_VerifyMFAWithBackupCode(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		mr := miniredis.RunT(t)
		r := newAuthRouterWithMFA(t, &mocks.MockUserRepository{}, &mocks.MockMFARepository{}, mr)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-backup-code",
			bytes.NewBufferString(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		mr := miniredis.RunT(t)
		r := newAuthRouterWithMFA(t, &mocks.MockUserRepository{}, &mocks.MockMFARepository{}, mr)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-backup-code",
			bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid or expired challenge", func(t *testing.T) {
		mr := miniredis.RunT(t)
		r := newAuthRouterWithMFA(t, &mocks.MockUserRepository{}, &mocks.MockMFARepository{}, mr)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-backup-code",
			bytes.NewBufferString(`{"challenge_id":"nonexistent","backup_code":"ABCD1234"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid backup code", func(t *testing.T) {
		mr := miniredis.RunT(t)
		userRepo := &mocks.MockUserRepository{}
		mfaRepo := &mocks.MockMFARepository{}
		user := testutil.CreateTestUser(authTestUserID, "test@test.com")
		userRepo.On("GetByID", mock.Anything, authTestUserID).Return(user, nil)
		// Code is hashed before lookup now, so match any (hashed) value.
		mfaRepo.On("GetBackupCode", mock.Anything, authTestUserID, mock.Anything).Return(nil, fmt.Errorf("not found"))

		require.NoError(t, mr.Set("mfa:challenge:ch-valid", authTestUserID))

		r := newAuthRouterWithMFA(t, userRepo, mfaRepo, mr)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify-backup-code",
			bytes.NewBufferString(`{"challenge_id":"ch-valid","backup_code":"WRONGCODE"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// newDeviceTestRouter wires the three device-credential endpoints. Production
// wiring lives in cmd/server/main.go; here we mount them directly so the
// handlers can be exercised in isolation.
func newDeviceTestRouter(t *testing.T, userRepo *mocks.MockUserRepository) *gin.Engine {
	t.Helper()
	h := buildAuthHandler(t, userRepo)
	r := gin.New()
	v1 := r.Group("/api/v1")
	auth := v1.Group("/auth")
	auth.POST("/device/login", h.DeviceLogin)
	auth.POST("/device/register", authContextMiddleware(authTestUserID, authTestSessionID), h.RegisterDevice)
	auth.DELETE("/device/:id", authContextMiddleware(authTestUserID, authTestSessionID), h.RevokeDevice)
	return r
}

func TestAuthHandler_RegisterDevice(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("CreateDeviceCredential", mock.Anything, mock.AnythingOfType("*models.DeviceCredential")).
		Return(nil)

	r := newDeviceTestRouter(t, userRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/device/register",
		jsonBody(t, map[string]interface{}{"install_id": "iOS-test", "platform": "ios"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	body := assertResponse(t, w, http.StatusOK, true)
	data := body["data"].(map[string]interface{})
	require.NotEmpty(t, data["credential"], "plaintext returned exactly once")
	require.NotEmpty(t, data["credential_id"])
	userRepo.AssertExpectations(t)
}

func TestAuthHandler_DeviceLogin(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	jwtSvc := services.NewJWTService(&authTestConfig().JWT)

	credPlain := "device-credential-plaintext"
	cred := &models.DeviceCredential{
		ID:             "cred-1",
		UserID:         authTestUserID,
		CredentialHash: jwtSvc.HashToken(credPlain),
	}
	user := testutil.CreateTestUser(authTestUserID, "test@example.com")

	userRepo.On("GetDeviceCredentialByHash", mock.Anything, mock.Anything).Return(cred, nil)
	userRepo.On("GetByID", mock.Anything, authTestUserID).Return(user, nil)
	userRepo.On("GetProfileByUserID", mock.Anything, authTestUserID).
		Return(testutil.CreateTestProfile(authTestUserID, "Test", "User"), nil)
	userRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*models.UserSession")).Return(nil)
	userRepo.On("UpdateLastLogin", mock.Anything, authTestUserID).Return(nil)
	userRepo.On("TouchDeviceCredential", mock.Anything, "cred-1").Return(nil)

	r := newDeviceTestRouter(t, userRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/device/login",
		jsonBody(t, map[string]interface{}{"credential": credPlain}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	body := assertResponse(t, w, http.StatusOK, true)
	data := body["data"].(map[string]interface{})
	tokens := data["tokens"].(map[string]interface{})
	require.NotEmpty(t, tokens["access_token"])
	require.NotEmpty(t, tokens["refresh_token"])
	userRepo.AssertExpectations(t)
}

func TestAuthHandler_DeviceLogin_RejectsBogus(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("GetDeviceCredentialByHash", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("not found"))

	r := newDeviceTestRouter(t, userRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/device/login",
		jsonBody(t, map[string]interface{}{"credential": "bogus"}))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_RevokeDevice(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("RevokeDeviceCredential", mock.Anything, mock.Anything, "cred-1").Return(nil)

	r := newDeviceTestRouter(t, userRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/auth/device/cred-1", nil)
	r.ServeHTTP(w, req)

	assertResponse(t, w, http.StatusOK, true)
	userRepo.AssertExpectations(t)
}

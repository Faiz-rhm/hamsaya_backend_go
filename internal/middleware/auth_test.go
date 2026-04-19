package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func getTestJWTConfig() *config.JWTConfig {
	return &config.JWTConfig{
		Secret:               "test-secret-key-at-least-32-characters-long-for-security",
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour,
	}
}

func newTestAuthMiddleware(userRepo *mocks.MockUserRepository) *AuthMiddleware {
	jwtSvc := services.NewJWTService(getTestJWTConfig())
	// nil tokenStorage — middleware checks nil before using it, falls through to DB
	return NewAuthMiddleware(jwtSvc, userRepo, nil, zap.NewNop())
}

// generateTestToken creates a valid JWT access token for testing.
func generateTestToken(userID, email string, aal int, sessionID string) string {
	jwtSvc := services.NewJWTService(getTestJWTConfig())
	tokenPair, _ := jwtSvc.GenerateTokenPair(userID, email, aal, sessionID)
	return tokenPair.AccessToken
}

// buildValidSession returns a UserSession whose AccessTokenHash matches token.
func buildValidSession(sessionID, userID, token string) *models.UserSession {
	jwtSvc := services.NewJWTService(getTestJWTConfig())
	return &models.UserSession{
		ID:              sessionID,
		UserID:          userID,
		AccessTokenHash: jwtSvc.HashToken(token),
		Revoked:         false,
		ExpiresAt:       time.Now().Add(time.Hour),
	}
}

// performRequest creates an HTTP request, optionally sets the Authorization
// header, runs it through the handler and returns the response recorder.
func performRequest(r http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// performRequestWithHeader does the same but lets the caller set the full
// Authorization header value (useful for malformed headers).
func performRequestWithHeader(r http.Handler, method, path, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// responseBody parses the JSON response body into a map for assertions.
func responseBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var body map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&body)
	return body
}

// ---------------------------------------------------------------------------
// TestRequireAuth
// ---------------------------------------------------------------------------

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-123"
		email     = "test@example.com"
		sessionID = "session-123"
	)

	tests := []struct {
		name           string
		setupRequest   func(router *gin.Engine, m *AuthMiddleware)
		setupMocks     func(userRepo *mocks.MockUserRepository, token string)
		token          string
		authHeader     string // overrides token when set
		wantStatus     int
		wantUserIDInCtx bool
	}{
		{
			name: "no authorization header",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid authorization format",
			authHeader: "Token sometoken",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty bearer token",
			authHeader: "Bearer ",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid jwt token",
			authHeader: "Bearer invalidtoken",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:  "valid token - user not found",
			token: generateTestToken(userID, email, models.AAL1, sessionID),
			setupMocks: func(userRepo *mocks.MockUserRepository, token string) {
				session := buildValidSession(sessionID, userID, token)
				userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)
				userRepo.On("GetByID", mock.Anything, userID).Return(nil, errors.New("user not found"))
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:  "valid token - user locked",
			token: generateTestToken(userID, email, models.AAL1, sessionID),
			setupMocks: func(userRepo *mocks.MockUserRepository, token string) {
				session := buildValidSession(sessionID, userID, token)
				userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

				lockedUser := testutil.CreateTestUser(userID, email)
				future := time.Now().Add(24 * time.Hour)
				lockedUser.LockedUntil = &future
				userRepo.On("GetByID", mock.Anything, userID).Return(lockedUser, nil)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:            "valid token - success",
			token:           generateTestToken(userID, email, models.AAL1, sessionID),
			setupMocks: func(userRepo *mocks.MockUserRepository, token string) {
				session := buildValidSession(sessionID, userID, token)
				userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

				normalUser := testutil.CreateTestUser(userID, email)
				userRepo.On("GetByID", mock.Anything, userID).Return(normalUser, nil)
			},
			wantStatus:      http.StatusOK,
			wantUserIDInCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)
			m := newTestAuthMiddleware(userRepo)

			// Determine the effective token string for mock setup.
			effectiveToken := tt.token
			tt.setupMocks(userRepo, effectiveToken)

			router := gin.New()
			router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
				userIDVal, _ := c.Get("user_id")
				c.JSON(http.StatusOK, gin.H{"user_id": userIDVal})
			})

			var w *httptest.ResponseRecorder
			if tt.authHeader != "" {
				w = performRequestWithHeader(router, http.MethodGet, "/test", tt.authHeader)
			} else {
				w = performRequest(router, http.MethodGet, "/test", tt.token)
			}

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantUserIDInCtx {
				body := responseBody(w)
				assert.Equal(t, userID, body["user_id"])
			}

			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestRequireAAL2
// ---------------------------------------------------------------------------

func TestRequireAAL2(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-123"
		email     = "test@example.com"
		sessionID = "session-aal-123"
	)

	tests := []struct {
		name       string
		aal        int
		wantStatus int
		wantBody   string
	}{
		{
			name:       "AAL1 token",
			aal:        models.AAL1,
			wantStatus: http.StatusForbidden,
			wantBody:   "multi-factor authentication",
		},
		{
			name:       "AAL2 token",
			aal:        models.AAL2,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateTestToken(userID, email, tt.aal, sessionID)

			userRepo := new(mocks.MockUserRepository)
			// verifySession always hits DB when tokenStorage is nil
			session := buildValidSession(sessionID, userID, token)
			userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

			m := newTestAuthMiddleware(userRepo)

			router := gin.New()
			router.GET("/test", m.RequireAAL2(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := performRequest(router, http.MethodGet, "/test", token)
			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestRequireAdmin
// ---------------------------------------------------------------------------

func TestRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		email     = "admin@example.com"
		sessionID = "session-admin-123"
	)

	tests := []struct {
		name       string
		role       models.UserRole
		wantStatus int
	}{
		{
			name:       "regular user",
			role:       models.RoleUser,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin user",
			role:       models.RoleAdmin,
			wantStatus: http.StatusOK,
		},
		{
			name:       "moderator user",
			role:       models.RoleModerator,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := "user-" + string(tt.role)
			token := generateTestToken(userID, email, models.AAL1, sessionID)

			userRepo := new(mocks.MockUserRepository)
			session := buildValidSession(sessionID, userID, token)
			userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

			u := testutil.CreateTestUser(userID, email)
			u.Role = tt.role
			userRepo.On("GetByID", mock.Anything, userID).Return(u, nil)

			m := newTestAuthMiddleware(userRepo)
			router := gin.New()
			router.GET("/admin", m.RequireAdmin(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := performRequest(router, http.MethodGet, "/admin", token)
			assert.Equal(t, tt.wantStatus, w.Code)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestRequireAdminOnly
// ---------------------------------------------------------------------------

func TestRequireAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		email     = "admin@example.com"
		sessionID = "session-adminonly-123"
	)

	tests := []struct {
		name       string
		role       models.UserRole
		wantStatus int
	}{
		{
			name:       "moderator user",
			role:       models.RoleModerator,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin user",
			role:       models.RoleAdmin,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := "user-" + string(tt.role)
			token := generateTestToken(userID, email, models.AAL1, sessionID)

			userRepo := new(mocks.MockUserRepository)
			session := buildValidSession(sessionID, userID, token)
			userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

			u := testutil.CreateTestUser(userID, email)
			u.Role = tt.role
			userRepo.On("GetByID", mock.Anything, userID).Return(u, nil)

			m := newTestAuthMiddleware(userRepo)
			router := gin.New()
			router.GET("/admin-only", m.RequireAdminOnly(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := performRequest(router, http.MethodGet, "/admin-only", token)
			assert.Equal(t, tt.wantStatus, w.Code)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestRequireVerifiedEmail
// ---------------------------------------------------------------------------

func TestRequireVerifiedEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-verified-123"
		email     = "user@example.com"
		sessionID = "session-verified-123"
	)

	tests := []struct {
		name          string
		emailVerified bool
		wantStatus    int
	}{
		{
			name:          "email not verified",
			emailVerified: false,
			wantStatus:    http.StatusForbidden,
		},
		{
			name:          "email verified",
			emailVerified: true,
			wantStatus:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateTestToken(userID, email, models.AAL1, sessionID)

			userRepo := new(mocks.MockUserRepository)
			session := buildValidSession(sessionID, userID, token)
			userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)

			u := testutil.CreateTestUser(userID, email)
			u.EmailVerified = tt.emailVerified
			userRepo.On("GetByID", mock.Anything, userID).Return(u, nil)

			m := newTestAuthMiddleware(userRepo)
			router := gin.New()
			router.GET("/verified", m.RequireVerifiedEmail(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			w := performRequest(router, http.MethodGet, "/verified", token)
			assert.Equal(t, tt.wantStatus, w.Code)
			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestOptionalAuth
// ---------------------------------------------------------------------------

func TestOptionalAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-optional-123"
		email     = "optional@example.com"
		sessionID = "session-optional-123"
	)

	tests := []struct {
		name            string
		authHeader      string
		token           string
		setupMocks      func(userRepo *mocks.MockUserRepository, token string)
		wantStatus      int
		wantUserIDInCtx bool
	}{
		{
			name:       "no token",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer thisisnotavalidjwt",
			setupMocks: func(_ *mocks.MockUserRepository, _ string) {},
			wantStatus: http.StatusOK,
		},
		{
			name:  "valid token",
			token: generateTestToken(userID, email, models.AAL1, sessionID),
			setupMocks: func(userRepo *mocks.MockUserRepository, token string) {
				session := buildValidSession(sessionID, userID, token)
				userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)
			},
			wantStatus:      http.StatusOK,
			wantUserIDInCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(mocks.MockUserRepository)

			effectiveToken := tt.token
			tt.setupMocks(userRepo, effectiveToken)

			m := newTestAuthMiddleware(userRepo)
			router := gin.New()
			router.GET("/optional", m.OptionalAuth(), func(c *gin.Context) {
				userIDVal, exists := c.Get("user_id")
				if exists {
					c.JSON(http.StatusOK, gin.H{"user_id": userIDVal})
				} else {
					c.JSON(http.StatusOK, gin.H{"user_id": nil})
				}
			})

			var w *httptest.ResponseRecorder
			if tt.authHeader != "" {
				w = performRequestWithHeader(router, http.MethodGet, "/optional", tt.authHeader)
			} else {
				w = performRequest(router, http.MethodGet, "/optional", tt.token)
			}

			assert.Equal(t, tt.wantStatus, w.Code)

			body := responseBody(w)
			if tt.wantUserIDInCtx {
				assert.Equal(t, userID, body["user_id"])
			} else {
				assert.Nil(t, body["user_id"])
			}

			userRepo.AssertExpectations(t)
		})
	}
}

// ---------------------------------------------------------------------------
// TestVerifySession — additional focused tests on session validation edge cases
// ---------------------------------------------------------------------------

func TestVerifySession_RevokedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-revoked-123"
		email     = "revoked@example.com"
		sessionID = "session-revoked-123"
	)

	token := generateTestToken(userID, email, models.AAL1, sessionID)

	userRepo := new(mocks.MockUserRepository)
	jwtSvc := services.NewJWTService(getTestJWTConfig())
	revokedSession := &models.UserSession{
		ID:              sessionID,
		UserID:          userID,
		AccessTokenHash: jwtSvc.HashToken(token),
		Revoked:         true, // session is revoked
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(revokedSession, nil)

	m := newTestAuthMiddleware(userRepo)
	router := gin.New()
	router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router, http.MethodGet, "/test", token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}

func TestVerifySession_ExpiredSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-expired-123"
		email     = "expired@example.com"
		sessionID = "session-expired-123"
	)

	token := generateTestToken(userID, email, models.AAL1, sessionID)

	userRepo := new(mocks.MockUserRepository)
	jwtSvc := services.NewJWTService(getTestJWTConfig())
	expiredSession := &models.UserSession{
		ID:              sessionID,
		UserID:          userID,
		AccessTokenHash: jwtSvc.HashToken(token),
		Revoked:         false,
		ExpiresAt:       time.Now().Add(-time.Hour), // already expired
	}
	userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(expiredSession, nil)

	m := newTestAuthMiddleware(userRepo)
	router := gin.New()
	router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router, http.MethodGet, "/test", token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}

func TestVerifySession_TokenHashMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-mismatch-123"
		email     = "mismatch@example.com"
		sessionID = "session-mismatch-123"
	)

	token := generateTestToken(userID, email, models.AAL1, sessionID)

	userRepo := new(mocks.MockUserRepository)
	mismatchSession := &models.UserSession{
		ID:              sessionID,
		UserID:          userID,
		AccessTokenHash: "this-is-not-the-hash-of-token", // deliberate mismatch
		Revoked:         false,
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(mismatchSession, nil)

	m := newTestAuthMiddleware(userRepo)
	router := gin.New()
	router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router, http.MethodGet, "/test", token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}

func TestVerifySession_SessionNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-nosession-123"
		email     = "nosession@example.com"
		sessionID = "session-nosession-123"
	)

	token := generateTestToken(userID, email, models.AAL1, sessionID)

	userRepo := new(mocks.MockUserRepository)
	userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(nil, errors.New("session not found"))

	m := newTestAuthMiddleware(userRepo)
	router := gin.New()
	router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router, http.MethodGet, "/test", token)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	userRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// TestRequireAuth_ContextValues — verify all context keys are set on success
// ---------------------------------------------------------------------------

func TestRequireAuth_ContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		userID    = "user-ctx-123"
		email     = "ctx@example.com"
		sessionID = "session-ctx-123"
	)

	token := generateTestToken(userID, email, models.AAL1, sessionID)

	userRepo := new(mocks.MockUserRepository)
	session := buildValidSession(sessionID, userID, token)
	userRepo.On("GetSessionByID", mock.Anything, sessionID).Return(session, nil)
	normalUser := testutil.CreateTestUser(userID, email)
	userRepo.On("GetByID", mock.Anything, userID).Return(normalUser, nil)

	m := newTestAuthMiddleware(userRepo)
	router := gin.New()

	var (
		ctxUserID    interface{}
		ctxEmail     interface{}
		ctxSessionID interface{}
		ctxAAL       interface{}
	)

	router.GET("/test", m.RequireAuth(), func(c *gin.Context) {
		ctxUserID, _ = c.Get("user_id")
		ctxEmail, _ = c.Get("email")
		ctxSessionID, _ = c.Get("session_id")
		ctxAAL, _ = c.Get("aal")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router, http.MethodGet, "/test", token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, ctxUserID)
	assert.Equal(t, email, ctxEmail)
	assert.Equal(t, sessionID, ctxSessionID)
	assert.Equal(t, models.AAL1, ctxAAL)

	userRepo.AssertExpectations(t)
}

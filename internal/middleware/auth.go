package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	jwtService *services.JWTService
	userRepo   repositories.UserRepository
	logger     *zap.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(
	jwtService *services.JWTService,
	userRepo repositories.UserRepository,
	logger *zap.Logger,
) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		userRepo:   userRepo,
		logger:     logger,
	}
}

// RequireAuth validates JWT token and adds user info to context
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			m.logger.Warn("Authentication failed", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Authentication required", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Add claims to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)

		c.Next()
	}
}

// RequireAAL2 requires AAL2 (MFA verified) authentication
func (m *AuthMiddleware) RequireAAL2() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, require authentication
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			m.logger.Warn("Authentication failed", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Authentication required", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Check AAL level
		if claims.AAL < models.AAL2 {
			m.logger.Warn("Insufficient authentication level",
				zap.String("user_id", claims.UserID),
				zap.Int("aal", claims.AAL),
			)
			utils.SendError(c, http.StatusForbidden,
				"This action requires multi-factor authentication",
				utils.ErrMFARequired)
			c.Abort()
			return
		}

		// Add claims to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)

		c.Next()
	}
}

// RequireVerifiedEmail requires the user's email to be verified
func (m *AuthMiddleware) RequireVerifiedEmail() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require authentication first
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			m.logger.Warn("Authentication failed", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Authentication required", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Get user from database
		user, err := m.userRepo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			m.logger.Error("Failed to get user", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Invalid user", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			m.logger.Warn("Email not verified",
				zap.String("user_id", user.ID),
				zap.String("email", user.Email),
			)
			utils.SendError(c, http.StatusForbidden,
				"Email verification required",
				utils.ErrEmailNotVerified)
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)
		c.Set("user", user)

		c.Next()
	}
}

// RequireAdmin requires the user to have admin role
func (m *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require authentication first
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			m.logger.Warn("Authentication failed", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Authentication required", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Get user from database to check role
		user, err := m.userRepo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			m.logger.Error("Failed to get user", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Invalid user", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Check if user is admin
		if !user.IsAdmin() {
			m.logger.Warn("Admin access denied",
				zap.String("user_id", user.ID),
				zap.String("role", string(user.Role)),
			)
			utils.SendError(c, http.StatusForbidden,
				"Admin access required",
				utils.NewForbiddenError("Admin access required", nil))
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)
		c.Set("user", user)

		c.Next()
	}
}

// RequireModerator requires the user to have moderator or admin role
func (m *AuthMiddleware) RequireModerator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require authentication first
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			m.logger.Warn("Authentication failed", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Authentication required", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Get user from database to check role
		user, err := m.userRepo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			m.logger.Error("Failed to get user", zap.Error(err))
			utils.SendError(c, http.StatusUnauthorized, "Invalid user", utils.ErrUnauthorized)
			c.Abort()
			return
		}

		// Check if user is moderator or admin
		if !user.IsAdminOrModerator() {
			m.logger.Warn("Moderator access denied",
				zap.String("user_id", user.ID),
				zap.String("role", string(user.Role)),
			)
			utils.SendError(c, http.StatusForbidden,
				"Moderator access required",
				utils.NewForbiddenError("Moderator access required", nil))
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)
		c.Set("user", user)

		c.Next()
	}
}

// OptionalAuth validates JWT token if present, but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := m.extractAndValidateToken(c)
		if err != nil {
			// Token is invalid or missing, but that's okay for optional auth
			// Just continue without setting user context
			c.Next()
			return
		}

		// Add claims to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("session_id", claims.SessionID)
		c.Set("aal", claims.AAL)

		c.Next()
	}
}

// extractAndValidateToken extracts and validates JWT from Authorization header
func (m *AuthMiddleware) extractAndValidateToken(c *gin.Context) (*models.JWTClaims, error) {
	// Get Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, utils.NewUnauthorizedError("Missing authorization header", nil)
	}

	// Check Bearer prefix
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, utils.NewUnauthorizedError("Invalid authorization header format", nil)
	}

	token := parts[1]
	if token == "" {
		return nil, utils.NewUnauthorizedError("Missing token", nil)
	}

	// Validate token
	claims, err := m.jwtService.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	// Verify session is still active
	if err := m.verifySession(c.Request.Context(), claims.SessionID, token); err != nil {
		return nil, err
	}

	return claims, nil
}

// verifySession checks if the session is still active and not revoked
func (m *AuthMiddleware) verifySession(ctx context.Context, sessionID, accessToken string) error {
	// Get session from database
	session, err := m.userRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		m.logger.Warn("Session not found",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
		return utils.NewUnauthorizedError("Invalid session", err)
	}

	// Check if session is revoked
	if session.Revoked {
		m.logger.Warn("Session has been revoked",
			zap.String("session_id", sessionID),
			zap.String("user_id", session.UserID),
		)
		return utils.NewUnauthorizedError("Session has been revoked", nil)
	}

	// Verify access token hash matches
	accessTokenHash := m.jwtService.HashToken(accessToken)
	if session.AccessTokenHash != accessTokenHash {
		m.logger.Warn("Access token hash mismatch",
			zap.String("session_id", sessionID),
			zap.String("user_id", session.UserID),
		)
		return utils.NewUnauthorizedError("Token mismatch", nil)
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		m.logger.Warn("Session has expired",
			zap.String("session_id", sessionID),
			zap.String("user_id", session.UserID),
			zap.Time("expires_at", session.ExpiresAt),
		)
		return utils.NewUnauthorizedError("Session has expired", nil)
	}

	return nil
}

// GetUserID returns the authenticated user ID from context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	return userID.(string), true
}

// GetSessionID returns the session ID from context
func GetSessionID(c *gin.Context) (string, bool) {
	sessionID, exists := c.Get("session_id")
	if !exists {
		return "", false
	}
	return sessionID.(string), true
}

// GetAAL returns the authentication assurance level from context
func GetAAL(c *gin.Context) (int, bool) {
	aal, exists := c.Get("aal")
	if !exists {
		return 0, false
	}
	return aal.(int), true
}

// GetUser returns the full user object from context (if loaded by middleware)
func GetUser(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	return user.(*models.User), true
}

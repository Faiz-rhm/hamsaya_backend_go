package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *services.AuthService
	validator   *utils.Validator
	logger      *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *services.AuthService, validator *utils.Validator, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validator:   validator,
		logger:      logger,
	}
}

// Register godoc
// @Summary Complete user profile registration
// @Description Creates complete user profile with firstname, lastname, and location. If user exists with incomplete profile, it will be updated and marked as complete. Requires email, password, first_name, last_name, latitude, and longitude.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Complete registration details with location"
// @Success 201 {object} utils.Response{data=models.AuthResponse} "User registered with complete profile"
// @Failure 400 {object} utils.Response
// @Failure 409 {object} utils.Response "User already has complete profile"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set request metadata
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	req.IPAddress = &ipAddress
	req.UserAgent = &userAgent

	response, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "User registered successfully", response)
}

// Login godoc
// @Summary Login or Auto-Register
// @Description Authenticate user with email and password. If user doesn't exist, automatically creates a new account with incomplete profile (is_complete: false). If user exists, performs normal login with password verification and MFA checks.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials (email and password only)"
// @Success 200 {object} utils.Response{data=models.AuthResponse} "Login successful or user auto-registered"
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response "Invalid credentials or account locked"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set request metadata
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	req.IPAddress = &ipAddress
	req.UserAgent = &userAgent

	response, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Check if MFA is required
	if response.RequiresMFA {
		utils.SendSuccess(c, http.StatusOK, "MFA verification required", response)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Login successful", response)
}

// AdminLogin godoc
// @Summary Admin login
// @Description Authenticate admin user with email and password. Only allows users with admin role to login.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Admin login credentials (email and password only)"
// @Success 200 {object} utils.Response{data=models.AuthResponse} "Admin login successful"
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response "Invalid credentials or account locked"
// @Failure 403 {object} utils.Response "Admin access required"
// @Router /admin/auth/login [post]
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req models.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set request metadata
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	req.IPAddress = &ipAddress
	req.UserAgent = &userAgent

	response, err := h.authService.AdminLogin(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Check if MFA is required
	if response.RequiresMFA {
		utils.SendSuccess(c, http.StatusOK, "MFA verification required", response)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Admin login successful", response)
}

// UnifiedAuth godoc
// @Summary Unified authentication (Login or Register)
// @Description Login if user exists, otherwise register new user with location
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.UnifiedAuthRequest true "Authentication details"
// @Success 200 {object} utils.Response{data=models.AuthResponse} "User logged in"
// @Success 201 {object} utils.Response{data=models.AuthResponse} "User registered"
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/unified [post]
func (h *AuthHandler) UnifiedAuth(c *gin.Context) {
	var req models.UnifiedAuthRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Set request metadata
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	req.IPAddress = &ipAddress
	req.UserAgent = &userAgent

	response, err := h.authService.UnifiedAuth(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Check if MFA is required
	if response.RequiresMFA {
		utils.SendSuccess(c, http.StatusOK, "MFA verification required", response)
		return
	}

	// Determine if this was a login or registration based on the response
	// (You could also track this in the service if needed)
	utils.SendSuccess(c, http.StatusOK, "Authentication successful", response)
}

// VerifyMFA godoc
// @Summary Verify MFA code
// @Description Verify MFA code during login
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.MFAVerifyChallengeRequest true "MFA verification"
// @Success 200 {object} utils.Response{data=models.AuthResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/mfa/verify [post]
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req models.MFAVerifyChallengeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	response, err := h.authService.VerifyMFA(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "MFA verification successful", response)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Get a new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} utils.Response{data=models.TokenPair}
// @Failure 401 {object} utils.Response
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	tokenPair, err := h.authService.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Token refreshed successfully", tokenPair)
}

// Logout godoc
// @Summary Logout
// @Description Logout from current session
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get session ID from context (set by auth middleware)
	sessionID, exists := c.Get("session_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "Session not found", utils.ErrUnauthorized)
		return
	}

	if err := h.authService.Logout(c.Request.Context(), sessionID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Logged out successfully", nil)
}

// LogoutAll godoc
// @Summary Logout from all devices
// @Description Logout from all active sessions
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/logout-all [post]
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	if err := h.authService.LogoutAll(c.Request.Context(), userID.(string)); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Logged out from all devices successfully", nil)
}

// VerifyEmail godoc
// @Summary Verify email
// @Description Verify user's email address
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.VerifyEmailRequest true "Verification token"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req models.VerifyEmailRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	if err := h.authService.VerifyEmail(c.Request.Context(), &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Email verified successfully", nil)
}

// ForgotPassword godoc
// @Summary Forgot password
// @Description Request password reset
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.ForgotPasswordRequest true "Email address"
// @Success 200 {object} utils.Response
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req models.ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	if err := h.authService.ForgotPassword(c.Request.Context(), &req); err != nil {
		h.handleError(c, err)
		return
	}

	// Always return success to prevent email enumeration
	utils.SendSuccess(c, http.StatusOK, "If an account exists with this email, a password reset link has been sent", nil)
}

// ResetPassword godoc
// @Summary Reset password
// @Description Reset password with reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req models.ResetPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Password reset successfully", nil)
}

// ChangePassword godoc
// @Summary Change password
// @Description Change password for authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.ChangePasswordRequest true "Current and new password"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var req models.ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID.(string), &req); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Password changed successfully", nil)
}

// GetActiveSessions godoc
// @Summary Get active sessions
// @Description Get all active sessions for the authenticated user
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]models.UserSession}
// @Failure 401 {object} utils.Response
// @Router /auth/sessions [get]
func (h *AuthHandler) GetActiveSessions(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	sessions, err := h.authService.GetActiveSessions(c.Request.Context(), userID.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Active sessions retrieved successfully", sessions)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *AuthHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Check for specific error types
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		utils.SendError(c, http.StatusNotFound, errMsg, err)
	case strings.Contains(errMsg, "already exists"):
		utils.SendError(c, http.StatusConflict, errMsg, err)
	case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "invalid"):
		utils.SendError(c, http.StatusUnauthorized, errMsg, err)
	default:
		h.logger.Error("Unhandled error in auth handler", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
	}
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		// Public routes
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/verify-email", h.VerifyEmail)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
		auth.POST("/mfa/verify", h.VerifyMFA)

		// Protected routes (require auth middleware)
		// These will be added with middleware in main.go
		// auth.POST("/logout", authMiddleware, h.Logout)
		// auth.POST("/logout-all", authMiddleware, h.LogoutAll)
		// auth.POST("/change-password", authMiddleware, h.ChangePassword)
		// auth.GET("/sessions", authMiddleware, h.GetActiveSessions)
	}
}

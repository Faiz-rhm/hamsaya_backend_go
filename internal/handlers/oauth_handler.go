package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// OAuthHandler handles OAuth authentication endpoints
type OAuthHandler struct {
	authService  *services.AuthService
	oauthService *services.OAuthService
	validator    *utils.Validator
	logger       *zap.Logger
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(
	authService *services.AuthService,
	oauthService *services.OAuthService,
	validator *utils.Validator,
	logger *zap.Logger,
) *OAuthHandler {
	return &OAuthHandler{
		authService:  authService,
		oauthService: oauthService,
		validator:    validator,
		logger:       logger,
	}
}

// GoogleOAuthRequest represents a Google OAuth request
type GoogleOAuthRequest struct {
	IDToken    string  `json:"id_token" validate:"required"`
	DeviceInfo *string `json:"device_info,omitempty"`
}

// FacebookOAuthRequest represents a Facebook OAuth request
type FacebookOAuthRequest struct {
	AccessToken string  `json:"access_token" validate:"required"`
	DeviceInfo  *string `json:"device_info,omitempty"`
}

// AppleOAuthRequest represents an Apple OAuth request
type AppleOAuthRequest struct {
	IDToken    string  `json:"id_token" validate:"required"`
	DeviceInfo *string `json:"device_info,omitempty"`
}

// GoogleOAuth godoc
// @Summary Google OAuth authentication
// @Description Authenticate or register a user using Google OAuth
// @Tags auth
// @Accept json
// @Produce json
// @Param request body GoogleOAuthRequest true "Google OAuth request"
// @Success 200 {object} utils.Response{data=models.AuthResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Router /auth/oauth/google [post]
func (h *OAuthHandler) GoogleOAuth(c *gin.Context) {
	var req GoogleOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Verify Google token and get user info
	oauthInfo, err := h.oauthService.VerifyGoogleToken(c.Request.Context(), req.IDToken)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Authenticate or create user
	user, profile, isNewUser, err := h.oauthService.AuthenticateWithOAuth(c.Request.Context(), oauthInfo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Generate tokens
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	tokens, err := h.authService.GenerateTokensForUser(
		c.Request.Context(),
		user,
		"AAL1", // OAuth doesn't require MFA
		req.DeviceInfo,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		h.logger.Error("Failed to generate tokens for OAuth user", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to generate authentication tokens", err)
		return
	}

	// Build response
	response := &models.AuthResponse{
		User: &models.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:           profile.ID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			Avatar:       profile.Avatar,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
			Country:      profile.Country,
			IsComplete:   profile.IsComplete,
		},
		Tokens:      tokens,
		RequiresMFA: false,
	}

	message := "Successfully authenticated with Google"
	if isNewUser {
		message = "Successfully registered with Google"
	}

	h.logger.Info("Google OAuth successful",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.Bool("new_user", isNewUser),
	)

	utils.SendSuccess(c, http.StatusOK, message, response)
}

// FacebookOAuth godoc
// @Summary Facebook OAuth authentication
// @Description Authenticate or register a user using Facebook OAuth
// @Tags auth
// @Accept json
// @Produce json
// @Param request body FacebookOAuthRequest true "Facebook OAuth request"
// @Success 200 {object} utils.Response{data=models.AuthResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Router /auth/oauth/facebook [post]
func (h *OAuthHandler) FacebookOAuth(c *gin.Context) {
	var req FacebookOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Verify Facebook token and get user info
	oauthInfo, err := h.oauthService.VerifyFacebookToken(c.Request.Context(), req.AccessToken)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Authenticate or create user
	user, profile, isNewUser, err := h.oauthService.AuthenticateWithOAuth(c.Request.Context(), oauthInfo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Generate tokens
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	tokens, err := h.authService.GenerateTokensForUser(
		c.Request.Context(),
		user,
		"AAL1",
		req.DeviceInfo,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		h.logger.Error("Failed to generate tokens for OAuth user", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to generate authentication tokens", err)
		return
	}

	// Build response
	response := &models.AuthResponse{
		User: &models.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:           profile.ID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			Avatar:       profile.Avatar,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
			Country:      profile.Country,
			IsComplete:   profile.IsComplete,
		},
		Tokens:      tokens,
		RequiresMFA: false,
	}

	message := "Successfully authenticated with Facebook"
	if isNewUser {
		message = "Successfully registered with Facebook"
	}

	h.logger.Info("Facebook OAuth successful",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.Bool("new_user", isNewUser),
	)

	utils.SendSuccess(c, http.StatusOK, message, response)
}

// AppleOAuth godoc
// @Summary Apple OAuth authentication
// @Description Authenticate or register a user using Apple OAuth
// @Tags auth
// @Accept json
// @Produce json
// @Param request body AppleOAuthRequest true "Apple OAuth request"
// @Success 200 {object} utils.Response{data=models.AuthResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Failure 501 {object} utils.Response
// @Router /auth/oauth/apple [post]
func (h *OAuthHandler) AppleOAuth(c *gin.Context) {
	var req AppleOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Verify Apple token and get user info
	oauthInfo, err := h.oauthService.VerifyAppleToken(c.Request.Context(), req.IDToken)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Authenticate or create user
	user, profile, isNewUser, err := h.oauthService.AuthenticateWithOAuth(c.Request.Context(), oauthInfo)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Generate tokens
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	tokens, err := h.authService.GenerateTokensForUser(
		c.Request.Context(),
		user,
		"AAL1",
		req.DeviceInfo,
		&ipAddress,
		&userAgent,
	)
	if err != nil {
		h.logger.Error("Failed to generate tokens for OAuth user", zap.Error(err))
		utils.SendError(c, http.StatusInternalServerError, "Failed to generate authentication tokens", err)
		return
	}

	// Build response
	response := &models.AuthResponse{
		User: &models.UserResponse{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			MFAEnabled:    user.MFAEnabled,
			CreatedAt:     user.CreatedAt,
		},
		Profile: &models.ProfileResponse{
			ID:           profile.ID,
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			Avatar:       profile.Avatar,
			Province:     profile.Province,
			District:     profile.District,
			Neighborhood: profile.Neighborhood,
			Country:      profile.Country,
			IsComplete:   profile.IsComplete,
		},
		Tokens:      tokens,
		RequiresMFA: false,
	}

	message := "Successfully authenticated with Apple"
	if isNewUser {
		message = "Successfully registered with Apple"
	}

	h.logger.Info("Apple OAuth successful",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
		zap.Bool("new_user", isNewUser),
	)

	utils.SendSuccess(c, http.StatusOK, message, response)
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *OAuthHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in OAuth handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}

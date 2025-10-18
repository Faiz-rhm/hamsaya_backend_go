package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/utils"
	"go.uber.org/zap"
)

// MFAHandler handles MFA-related endpoints
type MFAHandler struct {
	mfaService *services.MFAService
	validator  *utils.Validator
	logger     *zap.Logger
}

// NewMFAHandler creates a new MFA handler
func NewMFAHandler(mfaService *services.MFAService, validator *utils.Validator, logger *zap.Logger) *MFAHandler {
	return &MFAHandler{
		mfaService: mfaService,
		validator:  validator,
		logger:     logger,
	}
}

// EnrollTOTP godoc
// @Summary Enroll TOTP
// @Description Enroll TOTP (Time-based One-Time Password) for MFA
// @Tags mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.MFAEnrollRequest true "MFA enrollment request"
// @Success 200 {object} utils.Response{data=models.MFAEnrollResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Router /mfa/enroll [post]
func (h *MFAHandler) EnrollTOTP(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	email, exists := c.Get("email")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var req models.MFAEnrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Only TOTP is supported for now
	if req.Type != "TOTP" {
		utils.SendError(c, http.StatusBadRequest, "Only TOTP is currently supported", utils.ErrBadRequest)
		return
	}

	response, err := h.mfaService.EnrollTOTP(c.Request.Context(), userID.(string), email.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "TOTP enrollment initiated. Please verify with your authenticator app.", response)
}

// VerifyEnrollment godoc
// @Summary Verify MFA enrollment
// @Description Verify MFA enrollment with code from authenticator app
// @Tags mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.MFAVerifyRequest true "MFA verification request"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /mfa/verify-enrollment [post]
func (h *MFAHandler) VerifyEnrollment(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var req models.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	if err := h.mfaService.VerifyTOTPEnrollment(c.Request.Context(), userID.(string), req.FactorID, req.Code); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "MFA enabled successfully", nil)
}

// VerifyBackupCode godoc
// @Summary Verify with backup code
// @Description Verify MFA challenge with a backup code
// @Tags mfa
// @Accept json
// @Produce json
// @Param request body models.MFABackupCodeRequest true "Backup code request"
// @Success 200 {object} utils.Response{data=models.AuthResponse}
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /mfa/verify-backup-code [post]
func (h *MFAHandler) VerifyBackupCode(c *gin.Context) {
	var req models.MFABackupCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Get user ID from challenge (similar to VerifyMFA in auth service)
	// For now, this is handled in the auth service
	utils.SendError(c, http.StatusNotImplemented, "Backup code verification via this endpoint is not yet implemented", nil)
}

// DisableMFA godoc
// @Summary Disable MFA
// @Description Disable multi-factor authentication for the user
// @Tags mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.MFADisableRequest true "MFA disable request"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Router /mfa/disable [post]
func (h *MFAHandler) DisableMFA(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	var req models.MFADisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, "Invalid request body", utils.ErrInvalidJSON)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error(), utils.ErrValidation)
		return
	}

	// Disable MFA with password verification
	if err := h.mfaService.DisableMFA(c.Request.Context(), userID.(string), req.Password); err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "MFA disabled successfully", nil)
}

// RegenerateBackupCodes godoc
// @Summary Regenerate backup codes
// @Description Regenerate backup codes for MFA
// @Tags mfa
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=[]string}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /mfa/backup-codes/regenerate [post]
func (h *MFAHandler) RegenerateBackupCodes(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	codes, err := h.mfaService.RegenerateBackupCodes(c.Request.Context(), userID.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Backup codes regenerated successfully", gin.H{
		"backup_codes": codes,
	})
}

// GetBackupCodesCount godoc
// @Summary Get backup codes count
// @Description Get the count of unused backup codes
// @Tags mfa
// @Produce json
// @Security BearerAuth
// @Success 200 {object} utils.Response{data=int}
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /mfa/backup-codes/count [get]
func (h *MFAHandler) GetBackupCodesCount(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.SendError(c, http.StatusUnauthorized, "User not authenticated", utils.ErrUnauthorized)
		return
	}

	count, err := h.mfaService.GetBackupCodesCount(c.Request.Context(), userID.(string))
	if err != nil {
		h.handleError(c, err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Backup codes count retrieved", gin.H{
		"count": count,
	})
}

// handleError handles service errors and sends appropriate HTTP responses
func (h *MFAHandler) handleError(c *gin.Context, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*utils.AppError); ok {
		utils.SendError(c, appErr.Code, appErr.Message, appErr.Err)
		return
	}

	// Default to internal server error
	h.logger.Error("Unhandled error in MFA handler", zap.Error(err))
	utils.SendError(c, http.StatusInternalServerError, "An error occurred", err)
}

// RegisterRoutes registers MFA routes
func (h *MFAHandler) RegisterRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	mfa := router.Group("/mfa")
	{
		// All MFA routes require authentication
		mfa.POST("/enroll", authMiddleware, h.EnrollTOTP)
		mfa.POST("/verify-enrollment", authMiddleware, h.VerifyEnrollment)
		mfa.POST("/disable", authMiddleware, h.DisableMFA)
		mfa.POST("/backup-codes/regenerate", authMiddleware, h.RegenerateBackupCodes)
		mfa.GET("/backup-codes/count", authMiddleware, h.GetBackupCodesCount)

		// Public MFA verification endpoint (during login)
		// This is handled in auth routes: POST /auth/mfa/verify
	}
}

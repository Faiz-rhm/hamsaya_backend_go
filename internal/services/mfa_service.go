package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/repositories"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
)

const (
	BackupCodesCount  = 10
	BackupCodeLength  = 8
	TOTPSecretLength  = 32
	TOTPIssuer        = "Hamsaya"
	TOTPPeriod        = 30 // seconds
	TOTPDigits        = 6
)

// MFAService handles multi-factor authentication operations
type MFAService struct {
	mfaRepo         repositories.MFARepository
	userRepo        repositories.UserRepository
	passwordService *PasswordService
	logger          *zap.Logger
}

// NewMFAService creates a new MFA service
func NewMFAService(
	mfaRepo repositories.MFARepository,
	userRepo repositories.UserRepository,
	passwordService *PasswordService,
	logger *zap.Logger,
) *MFAService {
	return &MFAService{
		mfaRepo:         mfaRepo,
		userRepo:        userRepo,
		passwordService: passwordService,
		logger:          logger,
	}
}

// EnrollTOTP initiates TOTP enrollment for a user
func (s *MFAService) EnrollTOTP(ctx context.Context, userID, email string) (*models.MFAEnrollResponse, error) {
	// Check if user already has TOTP enabled
	factors, err := s.mfaRepo.GetFactorsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get MFA factors", zap.Error(err))
		return nil, utils.NewInternalError("Failed to enroll MFA", err)
	}

	// Check if TOTP is already enrolled
	for _, factor := range factors {
		if factor.Type == "TOTP" && factor.Status == "verified" {
			return nil, utils.NewConflictError("TOTP is already enrolled", nil)
		}
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: email,
		Period:      TOTPPeriod,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		s.logger.Error("Failed to generate TOTP secret", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate TOTP secret", err)
	}

	// Create MFA factor (unverified)
	factorID := uuid.New().String()
	now := time.Now()
	secret := key.Secret()

	factor := &models.MFAFactor{
		ID:        factorID,
		UserID:    userID,
		Type:      "TOTP",
		SecretKey: &secret,
		FactorID:  &factorID,
		Status:    "unverified",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.mfaRepo.CreateFactor(ctx, factor); err != nil {
		s.logger.Error("Failed to create MFA factor", zap.Error(err))
		return nil, utils.NewInternalError("Failed to enroll MFA", err)
	}

	// Generate QR code
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		s.logger.Error("Failed to generate QR code image", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate QR code", err)
	}

	if err := png.Encode(&buf, img); err != nil {
		s.logger.Error("Failed to encode QR code", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate QR code", err)
	}

	// Convert to base64 for easy transmission
	qrCodeDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Generate backup codes
	backupCodes, err := s.generateBackupCodes(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to generate backup codes", zap.Error(err))
		return nil, utils.NewInternalError("Failed to generate backup codes", err)
	}

	s.logger.Info("TOTP enrollment initiated",
		zap.String("user_id", userID),
		zap.String("factor_id", factorID),
	)

	return &models.MFAEnrollResponse{
		FactorID:    factorID,
		Type:        "TOTP",
		QRCodeURL:   qrCodeDataURL,
		SecretKey:   key.Secret(), // For manual entry
		BackupCodes: backupCodes,
	}, nil
}

// VerifyTOTPEnrollment verifies the TOTP code during enrollment
func (s *MFAService) VerifyTOTPEnrollment(ctx context.Context, userID, factorID, code string) error {
	// Get the MFA factor
	factor, err := s.mfaRepo.GetFactorByID(ctx, factorID)
	if err != nil {
		s.logger.Warn("Invalid factor ID", zap.Error(err))
		return utils.NewBadRequestError("Invalid factor ID", err)
	}

	// Verify it belongs to the user
	if factor.UserID != userID {
		s.logger.Warn("Factor does not belong to user",
			zap.String("user_id", userID),
			zap.String("factor_id", factorID),
		)
		return utils.NewUnauthorizedError("Invalid factor", nil)
	}

	// Check if already verified
	if factor.Status == "verified" {
		return utils.NewConflictError("Factor is already verified", nil)
	}

	// Verify TOTP code
	if factor.SecretKey == nil {
		return utils.NewInternalError("Factor secret not found", nil)
	}

	valid := totp.Validate(code, *factor.SecretKey)
	if !valid {
		s.logger.Warn("Invalid TOTP code",
			zap.String("user_id", userID),
			zap.String("factor_id", factorID),
		)
		return utils.NewBadRequestError("Invalid verification code", nil)
	}

	// Update factor status to verified
	if err := s.mfaRepo.UpdateFactorStatus(ctx, factorID, "verified"); err != nil {
		s.logger.Error("Failed to update factor status", zap.Error(err))
		return utils.NewInternalError("Failed to verify factor", err)
	}

	// Enable MFA for the user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return utils.NewInternalError("Failed to enable MFA", err)
	}

	user.MFAEnabled = true
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to enable MFA for user", zap.Error(err))
		return utils.NewInternalError("Failed to enable MFA", err)
	}

	s.logger.Info("TOTP verified and MFA enabled",
		zap.String("user_id", userID),
		zap.String("factor_id", factorID),
	)

	return nil
}

// VerifyTOTP verifies a TOTP code during login
func (s *MFAService) VerifyTOTP(ctx context.Context, userID, code string) (bool, error) {
	// Get user's verified TOTP factors
	factors, err := s.mfaRepo.GetFactorsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get MFA factors", zap.Error(err))
		return false, utils.NewInternalError("Failed to verify code", err)
	}

	// Find verified TOTP factor
	var totpFactor *models.MFAFactor
	for _, factor := range factors {
		if factor.Type == "TOTP" && factor.Status == "verified" {
			totpFactor = factor
			break
		}
	}

	if totpFactor == nil {
		s.logger.Warn("No verified TOTP factor found", zap.String("user_id", userID))
		return false, utils.NewBadRequestError("TOTP not enrolled", nil)
	}

	if totpFactor.SecretKey == nil {
		return false, utils.NewInternalError("Factor secret not found", nil)
	}

	// Verify TOTP code
	valid := totp.Validate(code, *totpFactor.SecretKey)
	if !valid {
		s.logger.Warn("Invalid TOTP code during login",
			zap.String("user_id", userID),
		)
		return false, nil
	}

	s.logger.Info("TOTP verified successfully",
		zap.String("user_id", userID),
	)

	return true, nil
}

// VerifyBackupCode verifies a backup code
func (s *MFAService) VerifyBackupCode(ctx context.Context, userID, code string) (bool, error) {
	// Normalize code (remove spaces, uppercase)
	normalizedCode := strings.ToUpper(strings.ReplaceAll(code, " ", ""))

	// Get backup code
	backupCode, err := s.mfaRepo.GetBackupCode(ctx, userID, normalizedCode)
	if err != nil {
		s.logger.Warn("Backup code not found",
			zap.String("user_id", userID),
		)
		return false, nil
	}

	// Check if already used
	if backupCode.Used {
		s.logger.Warn("Backup code already used",
			zap.String("user_id", userID),
			zap.String("code_id", backupCode.ID),
		)
		return false, utils.NewBadRequestError("Backup code has already been used", nil)
	}

	// Mark as used
	if err := s.mfaRepo.MarkBackupCodeAsUsed(ctx, backupCode.ID); err != nil {
		s.logger.Error("Failed to mark backup code as used", zap.Error(err))
		return false, utils.NewInternalError("Failed to verify backup code", err)
	}

	s.logger.Info("Backup code verified successfully",
		zap.String("user_id", userID),
		zap.String("code_id", backupCode.ID),
	)

	return true, nil
}

// DisableMFA disables MFA for a user with password verification
func (s *MFAService) DisableMFA(ctx context.Context, userID, password string) error {
	// Get user to verify password
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.Error(err))
		return utils.NewInternalError("Failed to disable MFA", err)
	}

	// Verify password
	if user.PasswordHash == nil {
		s.logger.Warn("User has no password (OAuth user)",
			zap.String("user_id", userID),
		)
		return utils.NewBadRequestError("Password verification not available for OAuth users", nil)
	}

	if !s.passwordService.Verify(password, *user.PasswordHash) {
		s.logger.Warn("Incorrect password for MFA disable",
			zap.String("user_id", userID),
		)
		return utils.NewUnauthorizedError("Incorrect password", nil)
	}

	// Get all user's factors
	factors, err := s.mfaRepo.GetFactorsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get MFA factors", zap.Error(err))
		return utils.NewInternalError("Failed to disable MFA", err)
	}

	// Delete all factors
	for _, factor := range factors {
		if err := s.mfaRepo.DeleteFactor(ctx, factor.ID); err != nil {
			s.logger.Error("Failed to delete MFA factor",
				zap.String("factor_id", factor.ID),
				zap.Error(err),
			)
			// Continue deleting others
		}
	}

	// Delete all backup codes
	if err := s.mfaRepo.DeleteAllBackupCodes(ctx, userID); err != nil {
		s.logger.Error("Failed to delete backup codes", zap.Error(err))
		// Continue anyway
	}

	// Disable MFA for user
	user.MFAEnabled = false
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to disable MFA for user", zap.Error(err))
		return utils.NewInternalError("Failed to disable MFA", err)
	}

	s.logger.Info("MFA disabled successfully", zap.String("user_id", userID))
	return nil
}

// RegenerateBackupCodes regenerates backup codes for a user
func (s *MFAService) RegenerateBackupCodes(ctx context.Context, userID string) ([]string, error) {
	// Delete existing backup codes
	if err := s.mfaRepo.DeleteAllBackupCodes(ctx, userID); err != nil {
		s.logger.Error("Failed to delete old backup codes", zap.Error(err))
		return nil, utils.NewInternalError("Failed to regenerate backup codes", err)
	}

	// Generate new backup codes
	codes, err := s.generateBackupCodes(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to generate backup codes", zap.Error(err))
		return nil, utils.NewInternalError("Failed to regenerate backup codes", err)
	}

	s.logger.Info("Backup codes regenerated", zap.String("user_id", userID))
	return codes, nil
}

// GetBackupCodesCount returns the count of unused backup codes
func (s *MFAService) GetBackupCodesCount(ctx context.Context, userID string) (int, error) {
	count, err := s.mfaRepo.GetUnusedBackupCodesCount(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get backup codes count", zap.Error(err))
		return 0, utils.NewInternalError("Failed to get backup codes count", err)
	}

	return count, nil
}

// generateBackupCodes generates and stores backup codes
func (s *MFAService) generateBackupCodes(ctx context.Context, userID string) ([]string, error) {
	codes := make([]string, BackupCodesCount)
	backupCodes := make([]*models.BackupCode, BackupCodesCount)

	for i := 0; i < BackupCodesCount; i++ {
		code, err := s.generateSecureCode(BackupCodeLength)
		if err != nil {
			return nil, err
		}

		codes[i] = code
		backupCodes[i] = &models.BackupCode{
			ID:        uuid.New().String(),
			UserID:    userID,
			Code:      code,
			Used:      false,
			CreatedAt: time.Now(),
		}
	}

	// Store backup codes in database
	if err := s.mfaRepo.CreateBackupCodes(ctx, backupCodes); err != nil {
		return nil, err
	}

	return codes, nil
}

// generateSecureCode generates a cryptographically secure random code
func (s *MFAService) generateSecureCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	for i := 0; i < length; i++ {
		b[i] = charset[int(b[i])%len(charset)]
	}

	// Format as XXXX-XXXX for 8-character codes
	code := string(b)
	if length == 8 {
		code = code[:4] + "-" + code[4:]
	}

	return code, nil
}

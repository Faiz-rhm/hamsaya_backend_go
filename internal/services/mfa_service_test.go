package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestMFAService(mfaRepo *mocks.MockMFARepository, userRepo *mocks.MockUserRepository) *MFAService {
	return NewMFAService(mfaRepo, userRepo, NewPasswordService(), zap.NewNop())
}

func newVerifiedTOTPFactor(userID, factorID string) *models.MFAFactor {
	secret := "JBSWY3DPEHPK3PXP" // fixed test secret
	now := time.Now()
	return &models.MFAFactor{
		ID:        factorID,
		UserID:    userID,
		Type:      "TOTP",
		SecretKey: &secret,
		Status:    "verified",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestMFAService_VerifyTOTP(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockMFARepository)
		code          string
		expectedValid bool
		expectedError string
	}{
		{
			name: "repo error",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				mfaRepo.On("GetFactorsByUserID", mock.Anything, "user-1").
					Return(nil, errors.New("db error"))
			},
			code:          "123456",
			expectedError: "db error",
		},
		{
			name: "no factors",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				mfaRepo.On("GetFactorsByUserID", mock.Anything, "user-1").
					Return([]*models.MFAFactor{}, nil)
			},
			code:          "123456",
			expectedError: "not enrolled",
		},
		{
			name: "no verified factor",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				secret := "JBSWY3DPEHPK3PXP"
				factor := &models.MFAFactor{
					ID: "f-1", UserID: "user-1", Type: "TOTP",
					SecretKey: &secret, Status: "unverified",
				}
				mfaRepo.On("GetFactorsByUserID", mock.Anything, "user-1").
					Return([]*models.MFAFactor{factor}, nil)
			},
			code:          "123456",
			expectedError: "not enrolled",
		},
		{
			name: "invalid code returns false no error",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				factor := newVerifiedTOTPFactor("user-1", "f-1")
				mfaRepo.On("GetFactorsByUserID", mock.Anything, "user-1").
					Return([]*models.MFAFactor{factor}, nil)
			},
			code:          "000000",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfaRepo := &mocks.MockMFARepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(mfaRepo)
			svc := newTestMFAService(mfaRepo, userRepo)

			valid, err := svc.VerifyTOTP(context.Background(), "user-1", tt.code)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValid, valid)
			}

			mfaRepo.AssertExpectations(t)
		})
	}
}

func TestMFAService_VerifyTOTPEnrollment(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockMFARepository, *mocks.MockUserRepository)
		userID        string
		factorID      string
		code          string
		expectedError string
	}{
		{
			name: "factor not found",
			setupMocks: func(mfaRepo *mocks.MockMFARepository, _ *mocks.MockUserRepository) {
				mfaRepo.On("GetFactorByID", mock.Anything, "f-bad").
					Return(nil, errors.New("not found"))
			},
			userID: "user-1", factorID: "f-bad", code: "123456",
			expectedError: "invalid factor",
		},
		{
			name: "factor belongs to different user",
			setupMocks: func(mfaRepo *mocks.MockMFARepository, _ *mocks.MockUserRepository) {
				factor := newVerifiedTOTPFactor("other-user", "f-1")
				mfaRepo.On("GetFactorByID", mock.Anything, "f-1").Return(factor, nil)
			},
			userID: "user-1", factorID: "f-1", code: "123456",
			expectedError: "invalid factor",
		},
		{
			name: "already verified",
			setupMocks: func(mfaRepo *mocks.MockMFARepository, _ *mocks.MockUserRepository) {
				factor := newVerifiedTOTPFactor("user-1", "f-1")
				mfaRepo.On("GetFactorByID", mock.Anything, "f-1").Return(factor, nil)
			},
			userID: "user-1", factorID: "f-1", code: "123456",
			expectedError: "already verified",
		},
		{
			name: "invalid code",
			setupMocks: func(mfaRepo *mocks.MockMFARepository, _ *mocks.MockUserRepository) {
				secret := "JBSWY3DPEHPK3PXP"
				factor := &models.MFAFactor{
					ID: "f-1", UserID: "user-1", Type: "TOTP",
					SecretKey: &secret, Status: "unverified",
				}
				mfaRepo.On("GetFactorByID", mock.Anything, "f-1").Return(factor, nil)
			},
			userID: "user-1", factorID: "f-1", code: "000000",
			expectedError: "invalid verification code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfaRepo := &mocks.MockMFARepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(mfaRepo, userRepo)
			svc := newTestMFAService(mfaRepo, userRepo)

			err := svc.VerifyTOTPEnrollment(context.Background(), tt.userID, tt.factorID, tt.code)

			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			mfaRepo.AssertExpectations(t)
		})
	}
}

func TestMFAService_DisableMFA(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockMFARepository, *mocks.MockUserRepository)
		password      string
		expectedError string
	}{
		{
			name: "user not found",
			setupMocks: func(_ *mocks.MockMFARepository, userRepo *mocks.MockUserRepository) {
				userRepo.On("GetByID", mock.Anything, "user-1").
					Return(nil, errors.New("not found"))
			},
			password:      "pass",
			expectedError: "failed to disable mfa",
		},
		{
			name: "oauth user has no password",
			setupMocks: func(_ *mocks.MockMFARepository, userRepo *mocks.MockUserRepository) {
				user := &models.User{ID: "user-1", Email: "test@example.com", PasswordHash: nil}
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
			},
			password:      "pass",
			expectedError: "not available for oauth",
		},
		{
			name: "wrong password",
			setupMocks: func(_ *mocks.MockMFARepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
			},
			password:      "wrongpassword",
			expectedError: "incorrect password",
		},
		{
			name: "success",
			setupMocks: func(mfaRepo *mocks.MockMFARepository, userRepo *mocks.MockUserRepository) {
				user := testutil.CreateTestUser("user-1", "test@example.com")
				user.PasswordHash = func() *string { s := testPasswordHash; return &s }()
				user.MFAEnabled = true
				userRepo.On("GetByID", mock.Anything, "user-1").Return(user, nil)
				factor := newVerifiedTOTPFactor("user-1", "f-1")
				mfaRepo.On("GetFactorsByUserID", mock.Anything, "user-1").
					Return([]*models.MFAFactor{factor}, nil)
				mfaRepo.On("DeleteFactor", mock.Anything, "f-1").Return(nil)
				mfaRepo.On("DeleteAllBackupCodes", mock.Anything, "user-1").Return(nil)
				userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			password: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfaRepo := &mocks.MockMFARepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(mfaRepo, userRepo)
			svc := newTestMFAService(mfaRepo, userRepo)

			err := svc.DisableMFA(context.Background(), "user-1", tt.password)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
			}

			mfaRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestMFAService_VerifyBackupCode(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockMFARepository)
		code          string
		expectedValid bool
		expectedError string
	}{
		{
			name: "code not found returns false",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				mfaRepo.On("GetBackupCode", mock.Anything, "user-1", "ABCD1234").
					Return(nil, errors.New("not found"))
			},
			code:          "ABCD1234",
			expectedValid: false,
		},
		{
			name: "already used",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				bc := &models.BackupCode{ID: "bc-1", UserID: "user-1", Code: "ABCD1234", Used: true}
				mfaRepo.On("GetBackupCode", mock.Anything, "user-1", "ABCD1234").Return(bc, nil)
			},
			code:          "ABCD1234",
			expectedError: "already been used",
		},
		{
			name: "success",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				bc := &models.BackupCode{ID: "bc-2", UserID: "user-1", Code: "EFGH5678", Used: false}
				mfaRepo.On("GetBackupCode", mock.Anything, "user-1", "EFGH5678").Return(bc, nil)
				mfaRepo.On("MarkBackupCodeAsUsed", mock.Anything, "bc-2").Return(nil)
			},
			code:          "EFGH5678",
			expectedValid: true,
		},
		{
			name: "normalizes lowercase input",
			setupMocks: func(mfaRepo *mocks.MockMFARepository) {
				bc := &models.BackupCode{ID: "bc-3", UserID: "user-1", Code: "ABCDEFGH", Used: false}
				mfaRepo.On("GetBackupCode", mock.Anything, "user-1", "ABCDEFGH").Return(bc, nil)
				mfaRepo.On("MarkBackupCodeAsUsed", mock.Anything, "bc-3").Return(nil)
			},
			code:          "abcdefgh",
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfaRepo := &mocks.MockMFARepository{}
			userRepo := new(mocks.MockUserRepository)
			tt.setupMocks(mfaRepo)
			svc := newTestMFAService(mfaRepo, userRepo)

			valid, err := svc.VerifyBackupCode(context.Background(), "user-1", tt.code)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValid, valid)
			}

			mfaRepo.AssertExpectations(t)
		})
	}
}

func TestMFAService_GetBackupCodesCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		userRepo := new(mocks.MockUserRepository)
		mfaRepo.On("GetUnusedBackupCodesCount", mock.Anything, "user-1").Return(7, nil)

		svc := newTestMFAService(mfaRepo, userRepo)
		count, err := svc.GetBackupCodesCount(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, 7, count)
		mfaRepo.AssertExpectations(t)
	})

	t.Run("repo error", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		userRepo := new(mocks.MockUserRepository)
		mfaRepo.On("GetUnusedBackupCodesCount", mock.Anything, "user-1").
			Return(0, errors.New("db error"))

		svc := newTestMFAService(mfaRepo, userRepo)
		_, err := svc.GetBackupCodesCount(context.Background(), "user-1")

		require.Error(t, err)
		mfaRepo.AssertExpectations(t)
	})
}

func TestMFAService_RegenerateBackupCodes(t *testing.T) {
	t.Run("delete fails", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		userRepo := new(mocks.MockUserRepository)
		mfaRepo.On("DeleteAllBackupCodes", mock.Anything, "user-1").
			Return(errors.New("db error"))

		svc := newTestMFAService(mfaRepo, userRepo)
		_, err := svc.RegenerateBackupCodes(context.Background(), "user-1")

		require.Error(t, err)
		mfaRepo.AssertExpectations(t)
	})

	t.Run("success generates 10 codes", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		userRepo := new(mocks.MockUserRepository)
		mfaRepo.On("DeleteAllBackupCodes", mock.Anything, "user-1").Return(nil)
		mfaRepo.On("CreateBackupCodes", mock.Anything, mock.AnythingOfType("[]*models.BackupCode")).Return(nil)

		svc := newTestMFAService(mfaRepo, userRepo)
		codes, err := svc.RegenerateBackupCodes(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Len(t, codes, BackupCodesCount)
		// each code should be XXXX-XXXX format
		for _, code := range codes {
			assert.Equal(t, 9, len(code), "expected XXXX-XXXX format")
			assert.Equal(t, '-', rune(code[4]))
		}
		mfaRepo.AssertExpectations(t)
	})
}

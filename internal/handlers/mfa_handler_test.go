package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/services"
	"github.com/hamsaya/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

const (
	mfaTestUserID  = "mfa-user-001"
	mfaTestEmail   = "mfa@example.com"
	mfaTestFactorID = "mfa-factor-001"
)

// mfaContextMiddleware injects both user_id and email (EnrollTOTP needs email).
func mfaContextMiddleware(userID, email string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("session_id", "mfa-sess-001")
		c.Set("email", email)
		c.Next()
	}
}

func newMFARouter(
	t *testing.T,
	mfaRepo *mocks.MockMFARepository,
	userRepo *mocks.MockUserRepository,
) *gin.Engine {
	t.Helper()
	svc := services.NewMFAService(mfaRepo, userRepo, services.NewPasswordService(), zap.NewNop())
	h := NewMFAHandler(svc, testutil.CreateTestValidator(), zap.NewNop())

	authed := mfaContextMiddleware(mfaTestUserID, mfaTestEmail)
	r := gin.New()
	r.POST("/api/v1/mfa/enroll", authed, h.EnrollTOTP)
	r.POST("/api/v1/mfa/verify-enrollment", authed, h.VerifyEnrollment)
	r.POST("/api/v1/mfa/disable", authed, h.DisableMFA)
	r.POST("/api/v1/mfa/backup-codes/regenerate", authed, h.RegenerateBackupCodes)
	r.GET("/api/v1/mfa/backup-codes/count", authed, h.GetBackupCodesCount)
	r.POST("/api/v1/mfa/verify-backup-code", h.VerifyBackupCode)

	r.POST("/api/v1/noauth/mfa/enroll", h.EnrollTOTP)
	return r
}

// --- EnrollTOTP ---

func TestMFAHandler_EnrollTOTP(t *testing.T) {
	t.Run("no user_id in context", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/noauth/mfa/enroll",
			strings.NewReader(`{"type":"TOTP"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/enroll", strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unsupported type (SMS)", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/enroll",
			strings.NewReader(`{"type":"SMS"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("TOTP enroll success", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		userRepo := &mocks.MockUserRepository{}
		userRepo.On("GetByID", mock.Anything, mfaTestUserID).
			Return(testutil.CreateTestUser(mfaTestUserID, mfaTestEmail), nil)
		mfaRepo.On("GetFactorsByUserID", mock.Anything, mfaTestUserID).Return([]*models.MFAFactor{}, nil)
		mfaRepo.On("CreateFactor", mock.Anything, mock.Anything).Return(nil)
		mfaRepo.On("CreateBackupCodes", mock.Anything, mock.Anything).Return(nil)

		r := newMFARouter(t, mfaRepo, userRepo)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/enroll",
			strings.NewReader(`{"type":"TOTP"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// --- VerifyEnrollment ---

func TestMFAHandler_VerifyEnrollment(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/verify-enrollment",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("factor not found", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		mfaRepo.On("GetFactorByID", mock.Anything, mfaTestFactorID).
			Return(nil, fmt.Errorf("not found"))
		r := newMFARouter(t, mfaRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/verify-enrollment",
			strings.NewReader(`{"factor_id":"`+mfaTestFactorID+`","code":"123456"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- DisableMFA ---

func TestMFAHandler_DisableMFA(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/disable",
			strings.NewReader(`not-json`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing password", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/disable",
			strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- VerifyBackupCode (always 501) ---

func TestMFAHandler_VerifyBackupCode(t *testing.T) {
	t.Run("returns not implemented", func(t *testing.T) {
		r := newMFARouter(t, &mocks.MockMFARepository{}, &mocks.MockUserRepository{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mfa/verify-backup-code",
			strings.NewReader(`{"challenge_id":"ch-001","backup_code":"ABCD1234"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotImplemented, w.Code)
	})
}

// --- GetBackupCodesCount ---

func TestMFAHandler_GetBackupCodesCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mfaRepo := &mocks.MockMFARepository{}
		mfaRepo.On("GetUnusedBackupCodesCount", mock.Anything, mfaTestUserID).Return(8, nil)
		r := newMFARouter(t, mfaRepo, &mocks.MockUserRepository{})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/mfa/backup-codes/count", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mfaRepo.AssertExpectations(t)
	})
}

package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func newBanRouter(adminRepo *mocks.MockAdminRepository) *gin.Engine {
	m := NewBanMiddleware(adminRepo, zap.NewNop())
	r := gin.New()
	r.Use(m.Enforce())
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestBanMiddleware_AllowsCleanIP(t *testing.T) {
	adminRepo := &mocks.MockAdminRepository{}
	adminRepo.On("IsIPBanned", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)

	r := newBanRouter(adminRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBanMiddleware_BlocksBannedIP(t *testing.T) {
	adminRepo := &mocks.MockAdminRepository{}
	adminRepo.On("IsIPBanned", mock.Anything, mock.AnythingOfType("string")).Return(true, nil)

	r := newBanRouter(adminRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "5.6.7.8")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestBanMiddleware_BlocksBannedDevice(t *testing.T) {
	adminRepo := &mocks.MockAdminRepository{}
	adminRepo.On("IsIPBanned", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
	adminRepo.On("IsDeviceBanned", mock.Anything, "device-abc").Return(true, nil)

	r := newBanRouter(adminRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	req.Header.Set("X-Device-ID", "device-abc")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestBanMiddleware_IPCheckError_Allows(t *testing.T) {
	// On error, ban check logs and continues (fail-open).
	adminRepo := &mocks.MockAdminRepository{}
	adminRepo.On("IsIPBanned", mock.Anything, mock.AnythingOfType("string")).Return(false, errors.New("redis down"))

	r := newBanRouter(adminRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBanMiddleware_NoDeviceHeader(t *testing.T) {
	adminRepo := &mocks.MockAdminRepository{}
	adminRepo.On("IsIPBanned", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
	// IsDeviceBanned must NOT be called when no X-Device-ID header present.

	r := newBanRouter(adminRepo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	adminRepo.AssertNotCalled(t, "IsDeviceBanned", mock.Anything, mock.Anything)
}

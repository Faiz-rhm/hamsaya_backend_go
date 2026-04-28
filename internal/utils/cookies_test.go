package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupCtx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func TestSetAdminAuthCookies_FlagsAndScope(t *testing.T) {
	c, w := setupCtx()
	cfg := NewCookieConfig("production", "admin.example.com")
	SetAdminAuthCookies(c, cfg, "atok", "rtok", "csrftok", 5*time.Minute, 7*24*time.Hour)

	got := w.Result().Cookies()
	byName := map[string]*http.Cookie{}
	for _, ck := range got {
		byName[ck.Name] = ck
	}

	access := byName[CookieAdminAccessToken]
	assert.NotNil(t, access)
	assert.Equal(t, "atok", access.Value)
	assert.True(t, access.HttpOnly)
	assert.True(t, access.Secure)
	assert.Equal(t, http.SameSiteLaxMode, access.SameSite)
	assert.Equal(t, "/", access.Path)
	assert.Equal(t, "admin.example.com", access.Domain)

	refresh := byName[CookieAdminRefreshToken]
	assert.NotNil(t, refresh)
	assert.True(t, refresh.HttpOnly)
	assert.Equal(t, "/api/v1/auth", refresh.Path, "refresh cookie must be path-scoped")

	csrf := byName[CookieCSRFToken]
	assert.NotNil(t, csrf)
	assert.False(t, csrf.HttpOnly, "CSRF cookie must be JS-readable")
	assert.True(t, csrf.Secure)
}

func TestNewCookieConfig_DevelopmentNotSecure(t *testing.T) {
	cfg := NewCookieConfig("development", "")
	assert.False(t, cfg.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cfg.SameSite)
}

func TestClearAdminAuthCookies_ExpiresAll(t *testing.T) {
	c, w := setupCtx()
	cfg := NewCookieConfig("production", "")
	ClearAdminAuthCookies(c, cfg)

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 3)
	for _, ck := range cookies {
		assert.Equal(t, -1, ck.MaxAge, "cookie %s must be expired", ck.Name)
	}
}

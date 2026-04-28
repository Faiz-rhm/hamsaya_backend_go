package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCSRFRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRF())
	r.GET("/safe", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/mutate", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCSRF_SafeMethodsBypass(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRF_BearerAuthBypass(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer some-token")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRF_RejectsMissingCookie(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
	req.Header.Set(utils.HeaderCSRF, "abc")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRF_RejectsMissingHeader(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: utils.CookieCSRFToken, Value: "abc"})
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRF_RejectsMismatch(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: utils.CookieCSRFToken, Value: "abc"})
	req.Header.Set(utils.HeaderCSRF, "xyz")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRF_AcceptsMatch(t *testing.T) {
	r := newCSRFRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: utils.CookieCSRFToken, Value: "abc"})
	req.Header.Set(utils.HeaderCSRF, "abc")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGenerateCSRFToken_HexAndUnique(t *testing.T) {
	a, err := utils.GenerateCSRFToken()
	require.NoError(t, err)
	b, err := utils.GenerateCSRFToken()
	require.NoError(t, err)
	assert.Len(t, a, 64)
	assert.Len(t, b, 64)
	assert.NotEqual(t, a, b)
}

package utils

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Cookie names used by the admin panel HttpOnly auth flow. The mobile app
// continues to authenticate via the Authorization header — these cookies are
// only set on responses to admin endpoints.
const (
	CookieAdminAccessToken  = "admin_token"
	CookieAdminRefreshToken = "admin_refresh_token"
	// CookieCSRFToken is intentionally NOT HttpOnly: the SPA reads it and
	// echoes its value back via the X-CSRF-Token header on mutating requests.
	// CSRF protection works because cross-origin attackers cannot read the
	// cookie's value (Same-Origin Policy on document.cookie), so they cannot
	// forge a matching header — even though the cookie is auto-attached.
	CookieCSRFToken = "csrf_token"

	// HeaderCSRF is the request header name CSRFMiddleware looks for.
	HeaderCSRF = "X-CSRF-Token"
)

// CookieConfig captures the deployment-aware flags applied to every admin
// auth cookie. Construct via NewCookieConfig() in the route wiring.
type CookieConfig struct {
	// Domain leaves Domain= unset when empty (host-only cookie).
	Domain string
	// Secure should be true in any non-development environment.
	Secure bool
	// SameSite defaults to http.SameSiteLaxMode when zero.
	SameSite http.SameSite
}

// NewCookieConfig returns sane defaults given a deployment env string
// ("development" / "staging" / "production"). Cookies emit Secure=false
// by default so they survive the http:// deployments Dokploy hands out
// before a TLS-capable domain is wired up (browsers silently drop
// Secure cookies on http:// origins, producing 'login works but page
// stays on /login'). Once the panel is reachable over HTTPS, set
// COOKIE_SECURE=true in the env panel to flip Secure back on. The old
// COOKIE_INSECURE=true escape hatch is still honoured for clarity.
func NewCookieConfig(env, domain string) CookieConfig {
	secure := false
	if v := os.Getenv("COOKIE_SECURE"); v == "true" || v == "1" {
		secure = true
	}
	if v := os.Getenv("COOKIE_INSECURE"); v == "true" || v == "1" {
		secure = false
	}
	return CookieConfig{
		Domain:   domain,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// SetAdminAuthCookies writes the access, refresh, and CSRF cookies onto the
// response. accessTTL/refreshTTL must be positive durations.
func SetAdminAuthCookies(c *gin.Context, cfg CookieConfig, accessToken, refreshToken, csrfToken string, accessTTL, refreshTTL time.Duration) {
	w := c.Writer

	http.SetCookie(w, &http.Cookie{
		Name:     CookieAdminAccessToken,
		Value:    accessToken,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   int(accessTTL.Seconds()),
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: cfg.SameSite,
	})

	// Refresh cookie scoped to the refresh endpoint to limit exposure surface.
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAdminRefreshToken,
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		Domain:   cfg.Domain,
		MaxAge:   int(refreshTTL.Seconds()),
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: cfg.SameSite,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieCSRFToken,
		Value:    csrfToken,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   int(accessTTL.Seconds()),
		Secure:   cfg.Secure,
		HttpOnly: false,
		SameSite: cfg.SameSite,
	})
}

// ClearAdminAuthCookies expires all three cookies so the browser drops them
// on receipt. Used by /auth/admin/logout.
func ClearAdminAuthCookies(c *gin.Context, cfg CookieConfig) {
	w := c.Writer
	for _, name := range []struct {
		name string
		path string
	}{
		{CookieAdminAccessToken, "/"},
		{CookieAdminRefreshToken, "/api/v1/auth"},
		{CookieCSRFToken, "/"},
	} {
		http.SetCookie(w, &http.Cookie{
			Name:     name.name,
			Value:    "",
			Path:     name.path,
			Domain:   cfg.Domain,
			MaxAge:   -1,
			Secure:   cfg.Secure,
			HttpOnly: name.name != CookieCSRFToken,
			SameSite: cfg.SameSite,
		})
	}
}

// GenerateCSRFToken returns a cryptographically random hex string suitable
// for the CSRF cookie+header pair. Length is fixed (32 bytes → 64 hex chars).
func GenerateCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/internal/utils"
)

// CSRF returns a Gin middleware enforcing the double-submit cookie CSRF
// pattern for cookie-authed requests:
//
//   - Safe methods (GET/HEAD/OPTIONS) pass through unchanged.
//   - Requests carrying an Authorization header are bypassed: Bearer tokens
//     are not auto-attached by browsers, so cross-origin forgery is impossible
//     and CSRF protection is unnecessary. This preserves the mobile API path.
//   - Cookie-authed mutating requests must present a non-empty csrf_token
//     cookie AND a matching X-CSRF-Token header (constant-time compared).
//     Mismatch or absence returns 403.
//
// Mount this middleware AFTER the auth middleware so the /auth/admin/login
// endpoint (which has no cookie yet) is not accidentally rejected.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		if c.GetHeader("Authorization") != "" {
			c.Next()
			return
		}

		cookie, err := c.Request.Cookie(utils.CookieCSRFToken)
		if err != nil || cookie.Value == "" {
			utils.SendError(c, http.StatusForbidden, "CSRF token cookie missing", utils.ErrForbidden)
			c.Abort()
			return
		}

		header := c.GetHeader(utils.HeaderCSRF)
		if header == "" {
			utils.SendError(c, http.StatusForbidden, "CSRF token header missing", utils.ErrForbidden)
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			utils.SendError(c, http.StatusForbidden, "CSRF token mismatch", utils.ErrForbidden)
			c.Abort()
			return
		}

		c.Next()
	}
}

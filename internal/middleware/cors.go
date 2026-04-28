package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/config"
)

// CORS returns a gin middleware that handles CORS
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed. When credentials are enabled the "*"
		// wildcard is unsafe and is never echoed — browsers reject
		// "Access-Control-Allow-Origin: *" combined with credentials, so a
		// "*" entry only acts as a fallback for non-credentialed requests
		// and the configured explicit origins are required for cookie auth.
		allowed := false
		wildcard := false
		for _, allowedOrigin := range cfg.AllowedOrigins {
			allowedOrigin = strings.TrimSpace(allowedOrigin)
			if allowedOrigin == "*" {
				wildcard = true
				continue
			}
			if allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else if wildcard && !cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))

		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

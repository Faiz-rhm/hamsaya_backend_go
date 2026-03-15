package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns middleware that sets essential security headers
// to protect against common web vulnerabilities (XSS, clickjacking, MIME sniffing, etc.)
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS filter (legacy browsers)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - restrict resource loading
		// Using 'self' as baseline; adjust based on your frontend needs
		c.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")

		// Prevent DNS prefetching to avoid privacy leaks
		c.Header("X-DNS-Prefetch-Control", "off")

		// Disable client-side caching for API responses by default
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		// Enable HSTS if behind TLS terminating proxy or direct HTTPS
		// Check X-Forwarded-Proto for reverse proxy setups
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			// max-age=1 year, include subdomains
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Permissions-Policy (formerly Feature-Policy) - restrict browser features
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

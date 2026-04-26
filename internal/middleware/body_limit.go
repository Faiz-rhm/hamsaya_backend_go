package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes caps the inbound request body for non-multipart requests.
// 5 MB is well above any JSON payload the API legitimately accepts (the largest
// is post creation with embedded location + tags, far under 100 KB) while
// keeping a comfortable margin for future fields. Multipart uploads have
// their own larger cap via gin.Engine.MaxMultipartMemory.
const DefaultMaxBodyBytes = 5 << 20

// BodyLimit installs http.MaxBytesReader on every request, returning
// 413 Request Entity Too Large when the inbound body exceeds maxBytes.
// Pair with gin.Engine.MaxMultipartMemory for file uploads.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

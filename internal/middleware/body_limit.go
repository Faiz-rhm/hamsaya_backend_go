package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes caps the inbound request body for non-multipart requests.
// 5 MB is well above any JSON payload the API legitimately accepts (the largest
// is post creation with embedded location + tags, far under 100 KB) while
// keeping a comfortable margin for future fields. Multipart uploads have
// their own larger cap via gin.Engine.MaxMultipartMemory.
const DefaultMaxBodyBytes = 5 << 20

// MaxUploadBodyBytes is the body cap used on multipart file-upload routes.
// Sized to fit the largest single asset we accept (videos up to 100 MB) plus
// multipart form overhead.
const MaxUploadBodyBytes = 110 << 20

// uploadPathPrefixes are request paths that bypass DefaultMaxBodyBytes and
// instead get MaxUploadBodyBytes. Keep this list narrow — only multipart
// upload endpoints belong here.
var uploadPathPrefixes = []string{
	"/api/v1/posts/upload-image",
	"/api/v1/users/me/avatar",
	"/api/v1/users/me/cover",
	"/api/v1/businesses/", // covers /:id/avatar /cover /attachments
	"/api/v1/chat/upload",
}

func isUploadPath(path string) bool {
	for _, p := range uploadPathPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// BodyLimit installs http.MaxBytesReader on every request, returning
// 413 Request Entity Too Large when the inbound body exceeds maxBytes.
// Upload routes (see [uploadPathPrefixes]) get [MaxUploadBodyBytes] instead
// of [maxBytes] so multipart media uploads aren't truncated mid-stream.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			cap := maxBytes
			if isUploadPath(c.Request.URL.Path) {
				cap = MaxUploadBodyBytes
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cap)
		}
		c.Next()
	}
}

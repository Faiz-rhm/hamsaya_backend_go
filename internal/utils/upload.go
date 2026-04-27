package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxImageUploadBytes is the per-file cap enforced by upload handlers.
// The global BodyLimit middleware caps the entire request; this is the
// per-attachment guard so a single oversized file gets a friendly error
// before WebP encoding wastes CPU.
const MaxImageUploadBytes = 25 * 1024 * 1024 // 25 MB

// MaxVideoUploadBytes is the per-file cap for video uploads.
const MaxVideoUploadBytes = 100 * 1024 * 1024 // 100 MB

// EnforceUploadSize aborts the request with 413 when the multipart file
// exceeds [maxBytes]. Returns true when the upload is acceptable, false
// when the response has already been written and the caller should bail.
//
// Pair the size check with [errIfNoFile] in the caller — this helper
// only validates size, not file presence.
func EnforceUploadSize(c *gin.Context, sizeBytes int64, maxBytes int64) bool {
	if sizeBytes <= maxBytes {
		return true
	}
	SendError(c,
		http.StatusRequestEntityTooLarge,
		"File too large — please upload a smaller file",
		ErrBadRequest,
	)
	return false
}

// Package handlers — storage_handler exposes a streaming proxy for MinIO
// objects under GET /api/v1/storage/<key>. Two callers consume it:
//
//   1. The admin panel's /storage/* proxy, when the admin server can't
//      reach MinIO directly (production behind a VPC, mixed-content over
//      HTTPS, etc.). Routing via the backend lets the admin reuse its
//      already-trusted API_URL hop.
//   2. Any client that wants a single canonical media origin (the API
//      host) instead of leaking the MinIO endpoint into URLs.
//
// The endpoint is PUBLIC — the bucket itself is public-read for s3:GetObject,
// so adding auth here wouldn't actually protect anything. Range requests
// are forwarded for <video> seek.
package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hamsaya/backend/pkg/storage"
	"go.uber.org/zap"
)

// StorageHandler streams MinIO objects.
type StorageHandler struct {
	storage *storage.Client
	logger  *zap.Logger
}

// NewStorageHandler wires the streaming proxy.
func NewStorageHandler(storageClient *storage.Client, logger *zap.Logger) *StorageHandler {
	return &StorageHandler{
		storage: storageClient,
		logger:  logger,
	}
}

// Stream handles GET /api/v1/storage/*key. The leading bucket prefix in
// the wildcard segment is optional — admin callers pass it through from
// the canonical URL, mobile callers pass just the key. Either resolves
// to the same MinIO object.
func (h *StorageHandler) Stream(c *gin.Context) {
	if h.storage == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	rawKey := c.Param("key")
	if rawKey == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	// Gin's catch-all keeps the leading slash; trim it.
	key := strings.TrimPrefix(rawKey, "/")
	if key == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	// Reject path traversal — segments can't contain `..`.
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			c.Status(http.StatusBadRequest)
			return
		}
	}

	// Strip a leading "<bucket>/" so the MinIO client sees only the key.
	// Mobile sends `post/abc.webp` directly; admin sends
	// `hamsaya-uploads/post/abc.webp`. Normalize to the same call.
	bucket := h.storage.BucketName()
	if bucket != "" && strings.HasPrefix(key, bucket+"/") {
		key = strings.TrimPrefix(key, bucket+"/")
	}

	rangeHeader := c.GetHeader("Range")
	obj, info, err := h.storage.StreamObject(c.Request.Context(), key, rangeHeader)
	if err != nil {
		h.logger.Warn("storage stream failed",
			zap.String("key", key),
			zap.Error(err),
		)
		c.Status(http.StatusNotFound)
		return
	}
	defer func() { _ = obj.Close() }()

	// Mirror upstream metadata so the browser caches correctly.
	if info.ContentType != "" {
		c.Header("Content-Type", info.ContentType)
	}
	if info.ETag != "" {
		c.Header("ETag", info.ETag)
	}
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=86400")

	// 206 Partial Content when the client asked for a range; minio-go
	// returns 200 with full body but the response still parses.
	status := http.StatusOK
	if rangeHeader != "" {
		status = http.StatusPartialContent
	}
	c.Status(status)

	if _, err := writeStream(c, obj); err != nil {
		// Connection closed mid-stream — common, not worth a warning.
		h.logger.Debug("storage stream write interrupted",
			zap.String("key", key),
			zap.Error(err),
		)
	}
}

// writeStream copies the upstream body into the response. Kept separate so
// the test layer can wrap a buffer reader without depending on gin internals.
func writeStream(c *gin.Context, r io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := c.Writer.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return total, nil
			}
			return total, err
		}
	}
}

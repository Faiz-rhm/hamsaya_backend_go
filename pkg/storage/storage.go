package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	_ "image/gif"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

// Client represents a storage client for S3/MinIO/R2
type Client struct {
	client     *minio.Client
	bucketName string
	cdnURL     string
	useSSL     bool
	endpoint   string
	pathStyle  bool
	logger     *zap.Logger
}

// Config holds storage configuration
type Config struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	BucketName string
	UseSSL     bool
	Region     string
	CDNURL     string
	// PathStyle controls public URL construction.
	//   true  → "{CDN_URL}/{bucket}/{key}" — MinIO host-as-base style
	//   false → "{CDN_URL}/{key}"          — bucket-scoped CDN (r2.dev, custom
	//                                        domain bound to a bucket)
	// Default in callers should be true for MinIO back-compat; set false for
	// Cloudflare R2 with a r2.dev or bound-domain CDN_URL.
	PathStyle bool
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	URL       string
	ThumbURL  string // ~240w variant for list cells / avatars
	MediumURL string // ~720w variant for cards
	Key       string
	Size      int64
	MimeType  string
	Width     int
	Height    int
}

// NewClient creates a new storage client
func NewClient(cfg *Config, logger *zap.Logger) (*Client, error) {
	// Initialize MinIO client
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	client := &Client{
		client:     minioClient,
		bucketName: cfg.BucketName,
		cdnURL:     normalizeCDNURL(cfg.CDNURL),
		useSSL:     cfg.UseSSL,
		endpoint:   cfg.Endpoint,
		pathStyle:  cfg.PathStyle,
		logger:     logger,
	}

	// Ensure bucket exists
	if err := client.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	logger.Info("Storage client initialized",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("bucket", cfg.BucketName),
		zap.Bool("use_ssl", cfg.UseSSL),
	)

	return client, nil
}

// Stats reports MinIO/S3 reachability and bucket identity for admin
// telemetry. Reachable is determined by a BucketExists probe — cheap
// HEAD-equivalent that doesn't enumerate objects.
type Stats struct {
	Reachable bool   `json:"reachable"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	UseSSL    bool   `json:"use_ssl"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// Stat probes the bucket and returns reachability + identity. Object count
// and total size are intentionally omitted — listing a multi-million-object
// bucket on every admin poll would be expensive.
func (c *Client) Stat(ctx context.Context) Stats {
	out := Stats{
		Endpoint: c.endpoint,
		Bucket:   c.bucketName,
		UseSSL:   c.useSSL,
	}
	start := time.Now()
	exists, err := c.client.BucketExists(ctx, c.bucketName)
	out.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Reachable = exists
	return out
}

// ensureBucket ensures the bucket exists and (for MinIO only) that its policy
// is set to public-read for s3:GetObject. Skips the policy step entirely for
// Cloudflare R2 — R2 doesn't implement the S3 PutBucketPolicy API and the
// call returns "NotImplemented". Public-read on R2 is enabled at the bucket
// level via the Cloudflare dashboard (r2.dev subdomain or bound custom
// domain), not via the S3 API.
func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		if err := c.client.MakeBucket(ctx, c.bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		c.logger.Info("Created storage bucket", zap.String("bucket", c.bucketName))
	}

	// Detect R2 by endpoint hostname; skip bucket policy on R2.
	if strings.Contains(c.endpoint, "r2.cloudflarestorage.com") {
		c.logger.Info("Skipping SetBucketPolicy on Cloudflare R2; enable public access via dashboard",
			zap.String("bucket", c.bucketName))
		return nil
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, c.bucketName)

	if err := c.client.SetBucketPolicy(ctx, c.bucketName, policy); err != nil {
		c.logger.Warn("Failed to apply public-read bucket policy", zap.Error(err))
	} else {
		c.logger.Info("Applied public-read bucket policy", zap.String("bucket", c.bucketName))
	}

	return nil
}

// UploadImage uploads an image file
func (c *Client) UploadImage(ctx context.Context, reader io.Reader, contentType, folder string) (*UploadResult, error) {
	// Read the image data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Decode image to get dimensions
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Generate object key
	ext := getExtensionFromFormat(format)
	id := uuid.New().String()
	filename := fmt.Sprintf("%s/%s%s", folder, id, ext)

	// Upload original
	size := int64(len(data))
	_, err = c.client.PutObject(ctx, c.bucketName, filename, bytes.NewReader(data), size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	bounds := img.Bounds()

	// Generate + upload thumb (240w) and medium (720w) variants. Best-effort —
	// failures don't block the main upload.
	thumbKey := fmt.Sprintf("%s/%s_thumb%s", folder, id, ext)
	mediumKey := fmt.Sprintf("%s/%s_medium%s", folder, id, ext)
	thumbURL := ""
	mediumURL := ""

	if thumb := ResizeImage(img, 240, 0); thumb != nil {
		if encoded, err := EncodeImage(thumb, format); err == nil {
			if buf, err := io.ReadAll(encoded); err == nil {
				if _, perr := c.client.PutObject(ctx, c.bucketName, thumbKey,
					bytes.NewReader(buf), int64(len(buf)),
					minio.PutObjectOptions{ContentType: contentType}); perr == nil {
					thumbURL = c.getPublicURL(thumbKey)
				} else {
					c.logger.Warn("thumb variant upload failed", zap.Error(perr))
				}
			}
		}
	}
	if medium := ResizeImage(img, 720, 0); medium != nil {
		if encoded, err := EncodeImage(medium, format); err == nil {
			if buf, err := io.ReadAll(encoded); err == nil {
				if _, perr := c.client.PutObject(ctx, c.bucketName, mediumKey,
					bytes.NewReader(buf), int64(len(buf)),
					minio.PutObjectOptions{ContentType: contentType}); perr == nil {
					mediumURL = c.getPublicURL(mediumKey)
				} else {
					c.logger.Warn("medium variant upload failed", zap.Error(perr))
				}
			}
		}
	}

	result := &UploadResult{
		URL:       c.getPublicURL(filename),
		ThumbURL:  thumbURL,
		MediumURL: mediumURL,
		Key:       filename,
		Size:      size,
		MimeType:  contentType,
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
	}

	c.logger.Info("Image uploaded",
		zap.String("key", filename),
		zap.String("url", result.URL),
		zap.Int64("size", size),
		zap.String("format", format),
	)

	return result, nil
}

// UploadFile uploads a generic file
func (c *Client) UploadFile(ctx context.Context, reader io.Reader, size int64, contentType, folder, filename string) (*UploadResult, error) {
	// Generate object key
	ext := filepath.Ext(filename)
	objectKey := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	// Upload to MinIO
	_, err := c.client.PutObject(ctx, c.bucketName, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	result := &UploadResult{
		URL:      c.getPublicURL(objectKey),
		Key:      objectKey,
		Size:     size,
		MimeType: contentType,
	}

	c.logger.Info("File uploaded",
		zap.String("key", objectKey),
		zap.String("url", result.URL),
		zap.Int64("size", size),
	)

	return result, nil
}

// StreamObject opens the object at the given key and returns a reader.
// Caller MUST close the reader. Forwards a Range header when supplied so
// <video> seek works through the admin proxy. ObjectInfo carries
// Content-Type / Content-Length / ETag for the HTTP response.
func (c *Client) StreamObject(ctx context.Context, key, rangeHeader string) (io.ReadCloser, minio.ObjectInfo, error) {
	opts := minio.GetObjectOptions{}
	if rangeHeader != "" {
		// minio-go's Set forwards the raw header. No error to check —
		// it just appends to the request headers map.
		opts.Set("Range", rangeHeader)
	}
	obj, err := c.client.GetObject(ctx, c.bucketName, key, opts)
	if err != nil {
		return nil, minio.ObjectInfo{}, fmt.Errorf("get object: %w", err)
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, minio.ObjectInfo{}, fmt.Errorf("stat object: %w", err)
	}
	return obj, info, nil
}

// BucketName returns the configured bucket name so callers (e.g. the admin
// stream handler) can strip a leading bucket prefix from a URL-derived key.
func (c *Client) BucketName() string {
	return c.bucketName
}

// Transcode fetches sourceKey, encodes it to format/quality, writes the
// result at targetKey, and removes the source on success. Satisfies the
// transcode.Encoder interface (without importing pkg/transcode — keeps
// dependency direction one-way).
func (c *Client) Transcode(ctx context.Context, sourceKey, targetKey, format string, quality int) error {
	obj, err := c.client.GetObject(ctx, c.bucketName, sourceKey, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("transcode get: %w", err)
	}
	defer func() { _ = obj.Close() }()

	img, _, err := ValidateImage(obj, 50*1024*1024)
	if err != nil {
		return fmt.Errorf("transcode decode: %w", err)
	}

	encoded, err := EncodeImage(img, format)
	if err != nil {
		return fmt.Errorf("transcode encode: %w", err)
	}

	// Buffer the encoded bytes so we can use the size in PutObject.
	buf, ok := encoded.(*bytes.Buffer)
	if !ok {
		var b bytes.Buffer
		if _, copyErr := io.Copy(&b, encoded); copyErr != nil {
			return fmt.Errorf("transcode buffer: %w", copyErr)
		}
		buf = &b
	}

	contentType := "image/" + format
	if _, err := c.client.PutObject(ctx, c.bucketName, targetKey, buf, int64(buf.Len()), minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return fmt.Errorf("transcode put: %w", err)
	}

	if err := c.client.RemoveObject(ctx, c.bucketName, sourceKey, minio.RemoveObjectOptions{}); err != nil {
		// Non-fatal — encoded variant exists, original lingering only costs storage.
		c.logger.Warn("transcode cleanup failed", zap.String("source_key", sourceKey), zap.Error(err))
	}

	c.logger.Info("transcode ok",
		zap.String("source", sourceKey),
		zap.String("target", targetKey),
		zap.String("format", format),
	)
	return nil
}

// Delete deletes a file from storage
func (c *Client) Delete(ctx context.Context, key string) error {
	err := c.client.RemoveObject(ctx, c.bucketName, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from storage: %w", err)
	}

	c.logger.Info("File deleted", zap.String("key", key))
	return nil
}

// DeleteByURL extracts the key from URL and deletes the file
func (c *Client) DeleteByURL(ctx context.Context, url string) error {
	key := c.extractKeyFromURL(url)
	if key == "" {
		return fmt.Errorf("invalid URL: cannot extract key")
	}
	return c.Delete(ctx, key)
}

// GetPresignedURL generates a presigned URL for secure uploads
func (c *Client) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	url, err := c.client.PresignedGetObject(ctx, c.bucketName, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}
	return url.String(), nil
}

// EnsureBucketInStorageURL rewrites a storage URL to include the bucket in the path if missing.
// Legacy URLs may be /post/xxx; MinIO path-style requires /bucketName/post/xxx.
func EnsureBucketInStorageURL(url, bucketName string) string {
	if url == "" || bucketName == "" {
		return url
	}
	idx := strings.Index(url, "://")
	if idx < 0 {
		return url
	}
	rest := url[idx+3:]
	pathIdx := strings.Index(rest, "/")
	if pathIdx < 0 {
		return url
	}
	path := rest[pathIdx:]
	if path == "" || path == "/" {
		return url
	}
	if strings.HasPrefix(path, "/"+bucketName+"/") || path == "/"+bucketName {
		return url
	}
	if strings.HasPrefix(path, "/post/") {
		return url[:idx+3+pathIdx] + "/" + bucketName + path
	}
	return url
}

// normalizeCDNURL strips trailing slashes and the legacy "/storage" path
// segment that operators sometimes paste into CDN_URL by analogy with
// the admin panel's /storage proxy. Backend URLs are MinIO path-style
// (base + "/" + bucket + "/" + key), so any extra path segment breaks
// the URL — guard against the common mistake at config-load time.
func normalizeCDNURL(raw string) string {
	out := strings.TrimRight(raw, "/")
	out = strings.TrimSuffix(out, "/storage")
	out = strings.TrimRight(out, "/")
	return out
}

// getPublicURL constructs the public URL for an object.
//
// Two layout modes:
//   - pathStyle=true:  "{base}/{bucket}/{key}" — MinIO-style.
//   - pathStyle=false: "{base}/{key}"          — bucket-scoped CDN (R2 r2.dev,
//                                                or a custom domain that
//                                                Cloudflare bound to one
//                                                bucket).
//
// When CDN_URL is empty (dev fallback), we always use path-style against the
// raw endpoint — the bucket-scoped CDN concept doesn't apply without a CDN.
func (c *Client) getPublicURL(key string) string {
	if c.cdnURL != "" {
		base := strings.TrimRight(c.cdnURL, "/")
		if c.pathStyle {
			return fmt.Sprintf("%s/%s/%s", base, c.bucketName, key)
		}
		return fmt.Sprintf("%s/%s", base, key)
	}
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	base := fmt.Sprintf("%s://%s", scheme, c.endpoint)
	return fmt.Sprintf("%s/%s/%s", base, c.bucketName, key)
}

// KeyFromURL is the exported wrapper around extractKeyFromURL so external
// commands (backfill scripts, transcoders) can map a stored URL back to its
// MinIO object key.
func (c *Client) KeyFromURL(url string) string {
	return c.extractKeyFromURL(url)
}

// PublicURL is the exported wrapper around getPublicURL so commands that mint
// new keys (re-encode, transcode) can construct the canonical URL we store
// in the DB.
func (c *Client) PublicURL(key string) string {
	return c.getPublicURL(key)
}

// extractKeyFromURL extracts the object key from a full URL.
//
// Accepts both layout modes (so historical URLs from a prior MinIO path-style
// deploy still resolve after switching to bucket-scoped R2 CDN):
//   - "{base}/{bucket}/{key}" → strips both base and bucket prefix
//   - "{base}/{key}"          → strips only base
func (c *Client) extractKeyFromURL(url string) string {
	path := ""
	if c.cdnURL != "" && strings.HasPrefix(url, c.cdnURL) {
		path = strings.TrimPrefix(strings.TrimPrefix(url, c.cdnURL), "/")
	} else {
		parts := strings.SplitN(url, "/", 4) // scheme, "", host, path
		if len(parts) >= 4 && parts[3] != "" {
			path = parts[3]
		}
	}
	if path == "" {
		return ""
	}
	prefix := c.bucketName + "/"
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	if path == c.bucketName {
		return ""
	}
	return path
}

// getExtensionFromFormat returns file extension from image format
func getExtensionFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	case "webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

// ValidateImage validates an image file
func ValidateImage(reader io.Reader, maxSize int64) (image.Image, string, error) {
	// Read data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image: %w", err)
	}

	// Check size
	if int64(len(data)) > maxSize {
		return nil, "", fmt.Errorf("image size exceeds maximum of %d bytes", maxSize)
	}

	// Decode image
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("invalid image format: %w", err)
	}

	return img, format, nil
}

// EncodeImage encodes an image to a specific format
func EncodeImage(img image.Image, format string) (io.Reader, error) {
	var buf bytes.Buffer

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "webp":
		// Configure WebP encoder options for high quality
		options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 90)
		if err != nil {
			return nil, fmt.Errorf("failed to create WebP encoder options: %w", err)
		}

		// Encode image to WebP format
		if err := webp.Encode(&buf, img, options); err != nil {
			return nil, fmt.Errorf("failed to encode WebP: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return &buf, nil
}

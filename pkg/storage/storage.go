package storage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// Client represents a storage client for S3/MinIO
type Client struct {
	client     *minio.Client
	bucketName string
	cdnURL     string
	useSSL     bool
	endpoint   string
	logger     *zap.Logger
}

// Config holds storage configuration
type Config struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	BucketName      string
	UseSSL          bool
	Region          string
	CDNURL          string
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	URL      string
	Key      string
	Size     int64
	MimeType string
	Width    int
	Height   int
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
		cdnURL:     cfg.CDNURL,
		useSSL:     cfg.UseSSL,
		endpoint:   cfg.Endpoint,
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

// ensureBucket ensures the bucket exists, creates it if not
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

		// Set bucket policy to public-read for uploaded files
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
			c.logger.Warn("Failed to set bucket policy", zap.Error(err))
		}
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
	filename := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	// Upload to MinIO
	size := int64(len(data))
	_, err = c.client.PutObject(ctx, c.bucketName, filename, bytes.NewReader(data), size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	// Get dimensions
	bounds := img.Bounds()

	result := &UploadResult{
		URL:      c.getPublicURL(filename),
		Key:      filename,
		Size:     size,
		MimeType: contentType,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
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

// getPublicURL constructs the public URL for an object
func (c *Client) getPublicURL(key string) string {
	// If CDN URL is configured, use it
	if c.cdnURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(c.cdnURL, "/"), key)
	}

	// Otherwise, construct MinIO URL
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, c.endpoint, c.bucketName, key)
}

// extractKeyFromURL extracts the object key from a full URL
func (c *Client) extractKeyFromURL(url string) string {
	// Handle CDN URLs
	if c.cdnURL != "" && strings.HasPrefix(url, c.cdnURL) {
		return strings.TrimPrefix(url, c.cdnURL+"/")
	}

	// Handle MinIO direct URLs
	// Format: http(s)://host/bucket/key
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		// Find bucket name and take everything after it
		for i, part := range parts {
			if part == c.bucketName && i+1 < len(parts) {
				return strings.Join(parts[i+1:], "/")
			}
		}
	}

	return ""
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
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return &buf, nil
}

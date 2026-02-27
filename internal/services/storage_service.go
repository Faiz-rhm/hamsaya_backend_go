package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"strings"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/storage"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

// ImageType represents the type of image being uploaded
type ImageType string

const (
	ImageTypeAvatar ImageType = "avatar"
	ImageTypeCover  ImageType = "cover"
	ImageTypePost   ImageType = "post"
)

// StorageService handles file storage operations
type StorageService struct {
	cfg       *config.Config
	logger    *zap.Logger
	client    *storage.Client
	processor *storage.ImageProcessor
}

// NewStorageService creates a new storage service
func NewStorageService(cfg *config.Config, logger *zap.Logger) *StorageService {
	// Create storage client (if storage is configured)
	var client *storage.Client
	if cfg.Storage.Endpoint != "" && cfg.Storage.AccessKey != "" {
		storageConfig := &storage.Config{
			Endpoint:   cfg.Storage.Endpoint,
			AccessKey:  cfg.Storage.AccessKey,
			SecretKey:  cfg.Storage.SecretKey,
			BucketName: cfg.Storage.BucketName,
			UseSSL:     cfg.Storage.UseSSL,
			Region:     cfg.Storage.Region,
			CDNURL:     cfg.Storage.CDNURL,
		}

		var err error
		client, err = storage.NewClient(storageConfig, logger)
		if err != nil {
			logger.Warn("Failed to initialize storage client, using mock storage",
				zap.Error(err),
			)
			client = nil
		}
	} else {
		logger.Info("Storage not configured, using mock storage")
	}

	return &StorageService{
		cfg:       cfg,
		logger:    logger,
		client:    client,
		processor: storage.NewImageProcessor(),
	}
}

// UploadImage uploads an image and returns photo metadata
func (s *StorageService) UploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, imageType ImageType) (*models.Photo, error) {
	// Validate file size (max 10MB)
	maxSize := int64(10 * 1024 * 1024)
	if header.Size > maxSize {
		return nil, utils.NewBadRequestError("File size exceeds 10MB limit", nil)
	}

	// Read image data first
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, utils.NewBadRequestError("Failed to read image file", err)
	}

	// Decode image to validate it and get format/dimensions
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		s.logger.Error("Failed to decode image", zap.Error(err))
		return nil, utils.NewBadRequestError("Invalid image file", err)
	}

	// Use the multipart Content-Type if valid; otherwise infer from decoded format
	contentType := header.Header.Get("Content-Type")
	if !s.isValidImageType(contentType) {
		contentType = mimeFromFormat(format)
		if !s.isValidImageType(contentType) {
			return nil, utils.NewBadRequestError(fmt.Sprintf("Invalid image type: %s. Only JPEG, PNG, and WebP are allowed", contentType), nil)
		}
	}

	// Process image based on type
	var processedImg image.Image
	switch imageType {
	case ImageTypeAvatar:
		// Process for avatar (crop to square, resize to 400x400)
		processedImg, err = s.processor.ProcessForAvatar(img, 400)
		if err != nil {
			return nil, utils.NewInternalError("Failed to process avatar image", err)
		}
	case ImageTypeCover:
		// Process for cover (resize to fit within 1600x900)
		processedImg, err = s.processor.ProcessForCover(img, 1600, 900)
		if err != nil {
			return nil, utils.NewInternalError("Failed to process cover image", err)
		}
	case ImageTypePost:
		// Process for post (resize to fit within 2048x2048)
		processedImg, err = s.processor.ProcessForPost(img)
		if err != nil {
			return nil, utils.NewInternalError("Failed to process post image", err)
		}
	default:
		processedImg = img
	}

	// Encode processed image
	reader, err := storage.EncodeImage(processedImg, format)
	if err != nil {
		return nil, utils.NewInternalError("Failed to encode image", err)
	}

	data, err = io.ReadAll(reader)
	if err != nil {
		return nil, utils.NewInternalError("Failed to read encoded image", err)
	}

	// Upload to storage
	var result *storage.UploadResult
	if s.client != nil {
		// Use real storage client
		result, err = s.client.UploadImage(ctx, bytes.NewReader(data), contentType, string(imageType))
		if err != nil {
			s.logger.Error("Failed to upload to storage", zap.Error(err))
			return nil, utils.NewInternalError("Failed to upload image", err)
		}
	} else {
		// Fall back to mock storage
		result = s.generateMockUploadResult(string(imageType), format, contentType, int64(len(data)), processedImg)
	}

	// Create photo model
	photo := &models.Photo{
		URL:      result.URL,
		Name:     header.Filename,
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
		MimeType: result.MimeType,
	}

	s.logger.Info("Image uploaded",
		zap.String("url", result.URL),
		zap.String("type", string(imageType)),
		zap.Int64("size", result.Size),
		zap.Int("width", result.Width),
		zap.Int("height", result.Height),
	)

	return photo, nil
}

// maxPostImageSize is the max size for post image uploads (10MB).
const maxPostImageSize = 10 * 1024 * 1024

// maxPostVideoSize is the max size for post video uploads (50MB).
const maxPostVideoSize = 50 * 1024 * 1024

// isVideoContentType returns true if contentType is a video type (e.g. "video/mp4").
func isVideoContentType(contentType string) bool {
	base := strings.TrimSpace(strings.Split(contentType, ";")[0])
	return strings.HasPrefix(base, "video/")
}

// UploadPostAttachment uploads an image or video for a post. Images are limited to 10MB and processed;
// videos are limited to 50MB and stored as-is.
func (s *StorageService) UploadPostAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*models.Photo, error) {
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if isVideoContentType(contentType) {
		// Video: larger limit, no image processing, upload raw file.
		if header.Size > maxPostVideoSize {
			return nil, utils.NewBadRequestError("Video file size exceeds 50MB limit", nil)
		}
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, utils.NewBadRequestError("Failed to read file", err)
		}
		size := int64(len(data))
		if size > maxPostVideoSize {
			return nil, utils.NewBadRequestError("Video file size exceeds 50MB limit", nil)
		}
		var result *storage.UploadResult
		if s.client != nil {
			result, err = s.client.UploadFile(ctx, bytes.NewReader(data), size, contentType, string(ImageTypePost), header.Filename)
			if err != nil {
				s.logger.Error("Failed to upload video to storage", zap.Error(err))
				return nil, utils.NewInternalError("Failed to upload video", err)
			}
		} else {
			// Mock: no dimensions for video
			result = &storage.UploadResult{
				URL:      fmt.Sprintf("https://storage.hamsaya.local/uploads/post/%s", header.Filename),
				Key:      "post/" + header.Filename,
				Size:     size,
				MimeType: contentType,
				Width:    0,
				Height:   0,
			}
		}
		return &models.Photo{
			URL:      result.URL,
			Name:     header.Filename,
			Size:     result.Size,
			Width:    result.Width,
			Height:   result.Height,
			MimeType: result.MimeType,
		}, nil
	}

	// Image: use existing 10MB limit and image processing.
	return s.UploadImage(ctx, file, header, ImageTypePost)
}

// DeleteImage deletes an image from storage
func (s *StorageService) DeleteImage(ctx context.Context, url string) error {
	if url == "" {
		return nil
	}

	if s.client != nil {
		// Use real storage client
		if err := s.client.DeleteByURL(ctx, url); err != nil {
			s.logger.Error("Failed to delete from storage", zap.Error(err), zap.String("url", url))
			return utils.NewInternalError("Failed to delete image", err)
		}
	}

	s.logger.Info("Image deleted", zap.String("url", url))
	return nil
}

// isValidImageType checks if the content type is a valid image type
func (s *StorageService) isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

// generateMockUploadResult generates a mock upload result when storage is not configured
func (s *StorageService) generateMockUploadResult(folder, format, contentType string, size int64, img image.Image) *storage.UploadResult {
	// Generate mock URL
	ext := getExtensionFromFormat(format)
	filename := fmt.Sprintf("%s/%s%s", folder, generateUUID(), ext)
	url := fmt.Sprintf("https://storage.hamsaya.local/uploads/%s", filename)

	// Get dimensions
	bounds := img.Bounds()

	return &storage.UploadResult{
		URL:      url,
		Key:      filename,
		Size:     size,
		MimeType: contentType,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
	}
}

// Helper functions

func mimeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

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

func generateUUID() string {
	// UUID v4 generation using crypto/rand
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ValidateImageDimensions validates image dimensions based on type
func (s *StorageService) ValidateImageDimensions(img image.Image, imageType ImageType) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	switch imageType {
	case ImageTypeAvatar:
		// Avatar should be square and at least 200x200
		if width < 200 || height < 200 {
			return utils.NewBadRequestError("Avatar must be at least 200x200 pixels", nil)
		}
	case ImageTypeCover:
		// Cover should be at least 1200x400
		if width < 1200 || height < 400 {
			return utils.NewBadRequestError("Cover image must be at least 1200x400 pixels", nil)
		}
	}

	return nil
}

// GetImageTypeFromString converts string to ImageType
func GetImageTypeFromString(s string) (ImageType, error) {
	switch strings.ToLower(s) {
	case "avatar":
		return ImageTypeAvatar, nil
	case "cover":
		return ImageTypeCover, nil
	case "post":
		return ImageTypePost, nil
	default:
		return "", fmt.Errorf("invalid image type: %s", s)
	}
}

// ReadImageFromMultipart reads an image from multipart form
func ReadImageFromMultipart(r io.Reader) (image.Image, string, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}

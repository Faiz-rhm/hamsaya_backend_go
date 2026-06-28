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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hamsaya/backend/config"
	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/internal/utils"
	"github.com/hamsaya/backend/pkg/nsfw"
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
	// ImageTypeAd forces WebP encoding regardless of source format. Ads are
	// served at fixed render slots in feeds where size matters more than
	// preserving the original codec.
	ImageTypeAd ImageType = "ad"
)

// StorageService handles file storage operations
type StorageService struct {
	cfg       *config.Config
	logger    *zap.Logger
	client    *storage.Client
	processor *storage.ImageProcessor
	// nsfwClient is optional. When non-nil, every uploaded image (decoded
	// bytes, pre-storage) is sent to the NSFW classifier sidecar. If the
	// classifier marks IsExplicit, the upload is rejected with 400. Wiring
	// is a single call to WithNSFWScanner from main.go.
	nsfwClient *nsfw.Client
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
			PathStyle:  cfg.Storage.PathStyle,
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

// WithNSFWScanner attaches a classifier client. Call once at startup
// after NewStorageService. Pass nil to disable scanning (default).
func (s *StorageService) WithNSFWScanner(c *nsfw.Client) *StorageService {
	s.nsfwClient = c
	return s
}

// scanForNSFW runs the optional NudeNet pass on raw image bytes. A
// scanner outage is non-fatal — we log and let the upload through so a
// flaky sidecar can't take down the whole upload pipeline.
func (s *StorageService) scanForNSFW(ctx context.Context, data []byte, filename, contentType string) error {
	if s.nsfwClient == nil {
		return nil
	}
	res := s.nsfwClient.Scan(ctx, data, filename, contentType)
	if res.ScannerError != nil {
		s.logger.Warn("nsfw scan skipped — scanner unavailable",
			zap.Error(res.ScannerError),
			zap.String("filename", filename),
		)
		return nil
	}
	if res.IsExplicit {
		s.logger.Warn("nsfw upload rejected",
			zap.String("top_class", res.TopClass),
			zap.Float64("top_score", res.TopScore),
			zap.String("filename", filename),
		)
		return utils.NewBadRequestError(
			"This image was flagged by our automated safety check and cannot be uploaded.",
			nil,
		)
	}
	return nil
}

// Client returns the underlying storage client. May be nil when storage
// isn't configured (mock mode). Used by the async transcode pool which
// needs direct access to fetch + put + delete by key.
func (s *StorageService) Client() *storage.Client {
	return s.client
}

// UploadImage uploads an image and returns photo metadata
func (s *StorageService) UploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, imageType ImageType) (*models.Photo, error) {
	const maxSize = int64(10 * 1024 * 1024) // 10 MB

	// Validate Content-Type header BEFORE reading bytes to reject non-images cheaply.
	contentType := header.Header.Get("Content-Type")
	if !s.isValidImageType(contentType) {
		return nil, utils.NewBadRequestError(
			fmt.Sprintf("Invalid image type: %s. Only JPEG, PNG, and WebP are allowed", contentType), nil)
	}

	// Use LimitReader so we never allocate more than maxSize+1 bytes regardless of
	// what the multipart header claims — header.Size can be spoofed by clients.
	limited := io.LimitReader(file, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, utils.NewBadRequestError("Failed to read image file", err)
	}
	if int64(len(data)) > maxSize {
		return nil, utils.NewBadRequestError("File size exceeds 10MB limit", nil)
	}

	// Decode image to validate it and get format/dimensions
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		s.logger.Error("Failed to decode image", zap.Error(err))
		return nil, utils.NewBadRequestError("Invalid image file", err)
	}

	// Re-check MIME using the actual decoded format in case the header was misleading
	if !s.isValidImageType(mimeFromFormat(format)) {
		return nil, utils.NewBadRequestError(
			fmt.Sprintf("Decoded image format %q is not allowed. Only JPEG, PNG, and WebP are accepted", format), nil)
	}

	// NSFW gate. Runs on the raw decoded bytes so the classifier sees the
	// same content the user submitted (before resize/recompress which
	// could lose detail). Skipped silently when the scanner isn't wired.
	if err := s.scanForNSFW(ctx, data, header.Filename, mimeFromFormat(format)); err != nil {
		return nil, err
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
	case ImageTypeAd:
		// Process for ad (resize to fit within 2048x2048, same as post). The
		// encode step below forces WebP regardless of source format.
		processedImg, err = s.processor.ProcessForPost(img)
		if err != nil {
			return nil, utils.NewInternalError("Failed to process ad image", err)
		}
	default:
		processedImg = img
	}

	// Force WebP for ads so served bytes are small regardless of source codec.
	encodeFormat := format
	if imageType == ImageTypeAd {
		encodeFormat = "webp"
		contentType = "image/webp"
	}

	// Encode processed image
	reader, err := storage.EncodeImage(processedImg, encodeFormat)
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
		URL:       result.URL,
		ThumbURL:  result.ThumbURL,
		MediumURL: result.MediumURL,
		Name:      header.Filename,
		Size:      result.Size,
		Width:     result.Width,
		Height:    result.Height,
		MimeType:  result.MimeType,
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

// maxPostVideoSize is the max size for post video uploads (50MB).
const maxPostVideoSize = 50 * 1024 * 1024

// maxPostAudioSize is the max size for voice message audio uploads (10MB).
const maxPostAudioSize = 10 * 1024 * 1024

// allowedAudioMimes is the allowlist of audio MIME types accepted for voice
// messages. Unlike video, we trust the client-declared type here because
// audio files are not rendered by browsers as images and carry no executable
// MIME confusion risk via CDN; size limit is the primary defence.
var allowedAudioMimes = map[string]struct{}{
	"audio/mp4":   {},
	"audio/x-m4a": {},
	"audio/aac":   {},
	"audio/mpeg":  {}, // mp3
	"audio/ogg":   {},
	"audio/webm":  {},
	"audio/wav":   {},
	"audio/x-wav": {},
}

// allowedVideoMimes is the strict allowlist of MIME types we re-derive from
// the file's first 512 bytes via http.DetectContentType. The client-supplied
// Content-Type is treated as a hint only — never as the authoritative type.
var allowedVideoMimes = map[string]struct{}{
	"video/mp4":       {},
	"video/quicktime": {},
	"video/webm":      {},
	"video/x-m4v":     {}, // some encoders mislabel mp4 as m4v; same container.
}

// isAudioContentType returns true if contentType is an audio type (e.g. "audio/mp4").
func isAudioContentType(contentType string) bool {
	base := strings.TrimSpace(strings.Split(contentType, ";")[0])
	return strings.HasPrefix(base, "audio/")
}

// isVideoContentType returns true if contentType is a video type (e.g. "video/mp4").
func isVideoContentType(contentType string) bool {
	base := strings.TrimSpace(strings.Split(contentType, ";")[0])
	return strings.HasPrefix(base, "video/")
}

// detectVideoMime sniffs the first 512 bytes via net/http's content-type
// detector and returns the resolved MIME if it's in our allowlist; "" otherwise.
// Defends against polyglot files labelled `video/mp4` that actually contain
// arbitrary executable payload.
func detectVideoMime(head []byte) string {
	mime := http.DetectContentType(head)
	base := strings.TrimSpace(strings.Split(mime, ";")[0])
	if _, ok := allowedVideoMimes[base]; ok {
		return base
	}
	return ""
}

// ffmpegFaststart rewrites an MP4 in-memory so the moov atom is at the front,
// allowing the player to start decoding immediately without downloading the
// whole file. Returns original data unchanged if ffmpeg is unavailable or fails.
func ffmpegFaststart(data []byte) []byte {
	in, err := os.CreateTemp("", "vid-in-*.mp4")
	if err != nil {
		return data
	}
	defer os.Remove(in.Name())
	if _, err := in.Write(data); err != nil {
		in.Close()
		return data
	}
	in.Close()

	out, err := os.CreateTemp("", "vid-out-*.mp4")
	if err != nil {
		return data
	}
	outName := out.Name()
	out.Close()
	defer os.Remove(outName)

	if err := exec.Command("ffmpeg", "-y", "-i", in.Name(), "-c", "copy", "-movflags", "+faststart", outName).Run(); err != nil {
		return data
	}
	processed, err := os.ReadFile(outName)
	if err != nil || len(processed) == 0 {
		return data
	}
	return processed
}

// ffmpegThumbnail extracts a JPEG thumbnail from the first second of a video.
// Returns nil if ffmpeg is unavailable or extraction fails.
func ffmpegThumbnail(data []byte) []byte {
	in, err := os.CreateTemp("", "vid-thumb-in-*.mp4")
	if err != nil {
		return nil
	}
	defer os.Remove(in.Name())
	if _, err := in.Write(data); err != nil {
		in.Close()
		return nil
	}
	in.Close()

	thumb, err := os.CreateTemp("", "vid-thumb-*.jpg")
	if err != nil {
		return nil
	}
	thumbName := thumb.Name()
	thumb.Close()
	defer os.Remove(thumbName)

	// Seek to 1s (or 0 if < 1s). Scale to 720px wide keeping aspect ratio.
	if err := exec.Command(
		"ffmpeg", "-y", "-ss", "00:00:01", "-i", in.Name(),
		"-vframes", "1", "-vf", "scale=720:-2", "-q:v", "3", thumbName,
	).Run(); err != nil {
		// Fallback: grab very first frame
		if err2 := exec.Command(
			"ffmpeg", "-y", "-i", in.Name(),
			"-vframes", "1", "-vf", "scale=720:-2", "-q:v", "3", thumbName,
		).Run(); err2 != nil {
			return nil
		}
	}
	thumbData, err := os.ReadFile(thumbName)
	if err != nil || len(thumbData) == 0 {
		return nil
	}
	return thumbData
}

// UploadPostAttachment uploads an image or video for a post. Images are limited to 10MB and processed;
// videos are limited to 50MB and stored as-is.
func (s *StorageService) UploadPostAttachment(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*models.Photo, error) {
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// If client sent a generic octet-stream, infer audio type from the file
	// extension. Dio's MultipartFile.fromFileSync may omit Content-Type for
	// certain audio extensions on some platforms.
	if contentType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		audioExtMimes := map[string]string{
			".m4a":  "audio/x-m4a",
			".aac":  "audio/aac",
			".mp3":  "audio/mpeg",
			".ogg":  "audio/ogg",
			".wav":  "audio/wav",
			".webm": "audio/webm",
		}
		if m, ok := audioExtMimes[ext]; ok {
			contentType = m
		}
	}

	if isVideoContentType(contentType) {
		// Video: larger limit, no image processing, upload raw file.
		// Use LimitReader to enforce the cap regardless of the (spoofable) header.Size.
		limited := io.LimitReader(file, maxPostVideoSize+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, utils.NewBadRequestError("Failed to read file", err)
		}
		size := int64(len(data))
		if size > maxPostVideoSize {
			return nil, utils.NewBadRequestError("Video file size exceeds 50MB limit", nil)
		}
		// Magic-byte enforcement: re-derive MIME from first 512 bytes. The
		// client-supplied Content-Type is just a hint — without this check
		// a polyglot file labelled `video/mp4` could be served via the CDN.
		head := data
		if len(head) > 512 {
			head = head[:512]
		}
		detected := detectVideoMime(head)
		if detected == "" {
			s.logger.Warn("Rejecting non-allowlisted video upload",
				zap.String("client_content_type", contentType),
				zap.Int("size", int(size)),
			)
			return nil, utils.NewBadRequestError("Unsupported video format", nil)
		}
		// Use the SERVER-DERIVED type for storage and metadata so attackers
		// can't poison the CDN response Content-Type.
		contentType = detected

		// Move moov atom to the front of the MP4 so players can start decoding
		// immediately without downloading the whole file (graceful no-op if
		// ffmpeg is absent).
		data = ffmpegFaststart(data)
		size = int64(len(data))

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

		photo := &models.Photo{
			URL:      result.URL,
			Name:     header.Filename,
			Size:     result.Size,
			Width:    result.Width,
			Height:   result.Height,
			MimeType: result.MimeType,
		}

		// Extract first-frame thumbnail so the mobile client can show a poster
		// image while the video initialises (eliminates the black-screen flash).
		if thumbData := ffmpegThumbnail(data); thumbData != nil && s.client != nil {
			thumbName := "thumb-" + header.Filename + ".jpg"
			thumbResult, thumbErr := s.client.UploadFile(
				ctx, bytes.NewReader(thumbData), int64(len(thumbData)),
				"image/jpeg", string(ImageTypePost), thumbName,
			)
			if thumbErr == nil {
				photo.ThumbURL = thumbResult.URL
			} else {
				s.logger.Warn("Video thumbnail upload failed", zap.Error(thumbErr))
			}
		}

		return photo, nil
	}

	// Audio (voice messages): store raw, 10MB cap, allowlist-validated MIME.
	if isAudioContentType(contentType) {
		mimeBase := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
		if _, ok := allowedAudioMimes[mimeBase]; !ok {
			return nil, utils.NewBadRequestError("Unsupported audio format", nil)
		}
		limited := io.LimitReader(file, maxPostAudioSize+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, utils.NewBadRequestError("Failed to read audio file", err)
		}
		if int64(len(data)) > maxPostAudioSize {
			return nil, utils.NewBadRequestError("Audio file size exceeds 10MB limit", nil)
		}
		size := int64(len(data))
		var result *storage.UploadResult
		if s.client != nil {
			result, err = s.client.UploadFile(ctx, bytes.NewReader(data), size, mimeBase, string(ImageTypePost), header.Filename)
			if err != nil {
				s.logger.Error("Failed to upload audio to storage", zap.Error(err))
				return nil, utils.NewInternalError("Failed to upload audio", err)
			}
		} else {
			result = &storage.UploadResult{
				URL:      fmt.Sprintf("https://storage.hamsaya.local/uploads/post/%s", header.Filename),
				Key:      "post/" + header.Filename,
				Size:     size,
				MimeType: mimeBase,
			}
		}
		return &models.Photo{
			URL:      result.URL,
			Name:     header.Filename,
			Size:     result.Size,
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

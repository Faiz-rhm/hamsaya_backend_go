package storage

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
)

// ImageProcessor handles image processing operations
type ImageProcessor struct {
	MaxWidth  int
	MaxHeight int
	Quality   int
}

// NewImageProcessor creates a new image processor with defaults
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		MaxWidth:  2048,
		MaxHeight: 2048,
		Quality:   90,
	}
}

// ResizeImage resizes an image to fit within max dimensions while maintaining aspect ratio
func (p *ImageProcessor) ResizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If image is already smaller than max dimensions, return as is
	if width <= maxWidth && height <= maxHeight {
		return img
	}

	// Calculate new dimensions maintaining aspect ratio
	ratio := float64(width) / float64(height)

	newWidth := maxWidth
	newHeight := int(float64(newWidth) / ratio)

	if newHeight > maxHeight {
		newHeight = maxHeight
		newWidth = int(float64(newHeight) * ratio)
	}

	return imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)
}

// CropToSquare crops an image to a square from the center
func (p *ImageProcessor) CropToSquare(img image.Image, size int) image.Image {
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}

// CreateThumbnail creates a thumbnail of specified size
func (p *ImageProcessor) CreateThumbnail(img image.Image, size int) image.Image {
	return imaging.Thumbnail(img, size, size, imaging.Lanczos)
}

// CompressJPEG compresses an image as JPEG with quality setting
func (p *ImageProcessor) CompressJPEG(img image.Image, quality int) (io.Reader, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("failed to compress JPEG: %w", err)
	}
	return &buf, nil
}

// CompressPNG compresses an image as PNG
func (p *ImageProcessor) CompressPNG(img image.Image) (io.Reader, error) {
	var buf bytes.Buffer
	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to compress PNG: %w", err)
	}
	return &buf, nil
}

// ProcessForAvatar processes an image for use as an avatar
// - Crops to square
// - Resizes to specified size
// - Compresses
func (p *ImageProcessor) ProcessForAvatar(img image.Image, size int) (image.Image, error) {
	// Crop to square from center
	squared := p.CropToSquare(img, img.Bounds().Dx())

	// Resize to target size
	resized := imaging.Resize(squared, size, size, imaging.Lanczos)

	return resized, nil
}

// ProcessForCover processes an image for use as a cover photo
// - Maintains aspect ratio
// - Resizes to fit within max dimensions
// - Compresses
func (p *ImageProcessor) ProcessForCover(img image.Image, maxWidth, maxHeight int) (image.Image, error) {
	// Resize maintaining aspect ratio
	resized := p.ResizeImage(img, maxWidth, maxHeight)

	return resized, nil
}

// ProcessForPost processes an image for use in a post
// - Maintains aspect ratio
// - Resizes to fit within max dimensions
// - Compresses
func (p *ImageProcessor) ProcessForPost(img image.Image) (image.Image, error) {
	// Resize to reasonable dimensions for posts
	resized := p.ResizeImage(img, p.MaxWidth, p.MaxHeight)

	return resized, nil
}

// ValidateDimensions validates image dimensions
func ValidateDimensions(img image.Image, minWidth, minHeight, maxWidth, maxHeight int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if minWidth > 0 && width < minWidth {
		return fmt.Errorf("image width %d is less than minimum %d", width, minWidth)
	}

	if minHeight > 0 && height < minHeight {
		return fmt.Errorf("image height %d is less than minimum %d", height, minHeight)
	}

	if maxWidth > 0 && width > maxWidth {
		return fmt.Errorf("image width %d exceeds maximum %d", width, maxWidth)
	}

	if maxHeight > 0 && height > maxHeight {
		return fmt.Errorf("image height %d exceeds maximum %d", height, maxHeight)
	}

	return nil
}

// ValidateAspectRatio validates image aspect ratio
func ValidateAspectRatio(img image.Image, minRatio, maxRatio float64) error {
	bounds := img.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	ratio := width / height

	if minRatio > 0 && ratio < minRatio {
		return fmt.Errorf("aspect ratio %.2f is less than minimum %.2f", ratio, minRatio)
	}

	if maxRatio > 0 && ratio > maxRatio {
		return fmt.Errorf("aspect ratio %.2f exceeds maximum %.2f", ratio, maxRatio)
	}

	return nil
}

// GetImageInfo returns information about an image
func GetImageInfo(img image.Image) map[string]interface{} {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	ratio := float64(width) / float64(height)

	return map[string]interface{}{
		"width":        width,
		"height":       height,
		"aspect_ratio": ratio,
	}
}

// Sharpen applies sharpening to an image
func (p *ImageProcessor) Sharpen(img image.Image, sigma float64) image.Image {
	return imaging.Sharpen(img, sigma)
}

// AdjustBrightness adjusts image brightness
func (p *ImageProcessor) AdjustBrightness(img image.Image, percentage float64) image.Image {
	return imaging.AdjustBrightness(img, percentage)
}

// AdjustContrast adjusts image contrast
func (p *ImageProcessor) AdjustContrast(img image.Image, percentage float64) image.Image {
	return imaging.AdjustContrast(img, percentage)
}

// Blur applies gaussian blur to an image
func (p *ImageProcessor) Blur(img image.Image, sigma float64) image.Image {
	return imaging.Blur(img, sigma)
}

// Rotate rotates an image by the specified angle
func (p *ImageProcessor) Rotate(img image.Image, angle float64) image.Image {
	return imaging.Rotate(img, angle, image.Black)
}

// FlipHorizontal flips an image horizontally
func (p *ImageProcessor) FlipHorizontal(img image.Image) image.Image {
	return imaging.FlipH(img)
}

// FlipVertical flips an image vertically
func (p *ImageProcessor) FlipVertical(img image.Image) image.Image {
	return imaging.FlipV(img)
}

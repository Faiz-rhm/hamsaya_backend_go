package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/hamsaya/backend/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testFile wraps *bytes.Reader to satisfy multipart.File (adds Close).
type testFile struct {
	*bytes.Reader
}

func (f *testFile) Close() error { return nil }

func makeTestFile(data []byte) multipart.File {
	return &testFile{bytes.NewReader(data)}
}

func makeHeader(filename, contentType string, size int64) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: filename,
		Size:     size,
		Header:   textproto.MIMEHeader{"Content-Type": []string{contentType}},
	}
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func newTestStorageService() *StorageService {
	return NewStorageService(&config.Config{}, zap.NewNop())
}

// --- UploadImage ---

func TestStorageService_UploadImage(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid content type rejected", func(t *testing.T) {
		svc := newTestStorageService()
		data := []byte("fake data")
		_, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("test.gif", "image/gif", int64(len(data))), ImageTypePost)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid image type")
	})

	t.Run("non-image content type rejected", func(t *testing.T) {
		svc := newTestStorageService()
		data := []byte("fake data")
		_, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("doc.pdf", "application/pdf", int64(len(data))), ImageTypePost)
		assert.Error(t, err)
	})

	t.Run("corrupted image data rejected", func(t *testing.T) {
		svc := newTestStorageService()
		data := []byte("not real image bytes")
		_, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("bad.jpg", "image/jpeg", int64(len(data))), ImageTypePost)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid image file")
	})

	t.Run("file too large rejected", func(t *testing.T) {
		svc := newTestStorageService()
		// 10MB + 1 byte of zeros — no need to be a real image, size check fires first
		data := make([]byte, 10*1024*1024+1)
		_, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("big.jpg", "image/jpeg", int64(len(data))), ImageTypePost)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "10MB")
	})

	t.Run("valid JPEG post image succeeds (mock storage)", func(t *testing.T) {
		svc := newTestStorageService()
		data := makeJPEG(t, 800, 600)
		photo, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("photo.jpg", "image/jpeg", int64(len(data))), ImageTypePost)
		require.NoError(t, err)
		assert.NotEmpty(t, photo.URL)
		assert.Equal(t, "photo.jpg", photo.Name)
		assert.Positive(t, photo.Width)
		assert.Positive(t, photo.Height)
		assert.Positive(t, photo.Size)
	})

	t.Run("valid PNG post image succeeds (mock storage)", func(t *testing.T) {
		svc := newTestStorageService()
		data := makePNG(t, 400, 400)
		photo, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("image.png", "image/png", int64(len(data))), ImageTypePost)
		require.NoError(t, err)
		assert.NotEmpty(t, photo.URL)
		assert.Equal(t, "image.png", photo.Name)
	})

	t.Run("avatar image processed to square", func(t *testing.T) {
		svc := newTestStorageService()
		data := makeJPEG(t, 600, 400) // non-square input
		photo, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("avatar.jpg", "image/jpeg", int64(len(data))), ImageTypeAvatar)
		require.NoError(t, err)
		assert.Equal(t, photo.Width, photo.Height, "avatar must be square")
		assert.LessOrEqual(t, photo.Width, 400)
	})

	t.Run("cover image processed within 1600x900", func(t *testing.T) {
		svc := newTestStorageService()
		data := makeJPEG(t, 3000, 2000)
		photo, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("cover.jpg", "image/jpeg", int64(len(data))), ImageTypeCover)
		require.NoError(t, err)
		assert.LessOrEqual(t, photo.Width, 1600)
		assert.LessOrEqual(t, photo.Height, 900)
	})

	t.Run("post image processed within 2048x2048", func(t *testing.T) {
		svc := newTestStorageService()
		data := makeJPEG(t, 4000, 3000)
		photo, err := svc.UploadImage(ctx, makeTestFile(data),
			makeHeader("post.jpg", "image/jpeg", int64(len(data))), ImageTypePost)
		require.NoError(t, err)
		assert.LessOrEqual(t, photo.Width, 2048)
		assert.LessOrEqual(t, photo.Height, 2048)
	})
}

// --- UploadPostAttachment ---

func TestStorageService_UploadPostAttachment(t *testing.T) {
	ctx := context.Background()

	t.Run("video bypasses image processing", func(t *testing.T) {
		svc := newTestStorageService()
		// Real MP4 ftyp atom — http.DetectContentType requires legitimate
		// magic bytes to classify as video/mp4. The bytes below are the
		// minimum valid ISO base media file format header (mp42 brand).
		data := []byte{
			0x00, 0x00, 0x00, 0x18, // box size = 24
			'f', 't', 'y', 'p',
			'm', 'p', '4', '2', // major brand
			0x00, 0x00, 0x00, 0x00, // minor version
			'm', 'p', '4', '2',
			'i', 's', 'o', 'm', // compatible brands
		}
		photo, err := svc.UploadPostAttachment(ctx, makeTestFile(data),
			makeHeader("video.mp4", "video/mp4", int64(len(data))))
		require.NoError(t, err)
		assert.NotEmpty(t, photo.URL)
		assert.Equal(t, "video/mp4", photo.MimeType)
		assert.Equal(t, 0, photo.Width)
		assert.Equal(t, 0, photo.Height)
	})

	t.Run("video with bogus magic bytes rejected", func(t *testing.T) {
		// New defence: client-supplied Content-Type=video/mp4 but the bytes
		// don't match — could be a polyglot. Must be rejected.
		svc := newTestStorageService()
		data := []byte("definitely not a real video file")
		_, err := svc.UploadPostAttachment(ctx, makeTestFile(data),
			makeHeader("fake.mp4", "video/mp4", int64(len(data))))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Unsupported video format")
	})

	t.Run("video too large rejected", func(t *testing.T) {
		svc := newTestStorageService()
		data := make([]byte, 50*1024*1024+1)
		_, err := svc.UploadPostAttachment(ctx, makeTestFile(data),
			makeHeader("big.mp4", "video/mp4", int64(len(data))))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "50MB")
	})

	t.Run("image delegates to UploadImage", func(t *testing.T) {
		svc := newTestStorageService()
		data := makeJPEG(t, 500, 500)
		photo, err := svc.UploadPostAttachment(ctx, makeTestFile(data),
			makeHeader("photo.jpg", "image/jpeg", int64(len(data))))
		require.NoError(t, err)
		assert.NotEmpty(t, photo.URL)
		assert.Positive(t, photo.Width)
	})

	t.Run("empty content type treated as octet-stream (not image)", func(t *testing.T) {
		svc := newTestStorageService()
		data := []byte("raw bytes")
		header := &multipart.FileHeader{
			Filename: "file.bin",
			Size:     int64(len(data)),
			Header:   textproto.MIMEHeader{},
		}
		// No Content-Type header — falls through to UploadImage with application/octet-stream
		// which is invalid image type → error
		_, err := svc.UploadPostAttachment(ctx, makeTestFile(data), header)
		assert.Error(t, err)
	})
}

// --- DeleteImage ---

func TestStorageService_DeleteImage(t *testing.T) {
	ctx := context.Background()

	t.Run("empty URL is no-op", func(t *testing.T) {
		svc := newTestStorageService()
		err := svc.DeleteImage(ctx, "")
		assert.NoError(t, err)
	})

	t.Run("non-empty URL with no client succeeds silently", func(t *testing.T) {
		svc := newTestStorageService()
		err := svc.DeleteImage(ctx, "https://storage.hamsaya.local/uploads/avatar/test.jpg")
		assert.NoError(t, err)
	})
}

// --- ValidateImageDimensions ---

func TestStorageService_ValidateImageDimensions(t *testing.T) {
	svc := newTestStorageService()

	makeImg := func(w, h int) image.Image {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}

	t.Run("avatar too small rejected", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(100, 100), ImageTypeAvatar)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "200x200")
	})

	t.Run("avatar exactly minimum accepted", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(200, 200), ImageTypeAvatar)
		assert.NoError(t, err)
	})

	t.Run("avatar large accepted", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(1000, 1000), ImageTypeAvatar)
		assert.NoError(t, err)
	})

	t.Run("cover too small rejected", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(800, 300), ImageTypeCover)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1200x400")
	})

	t.Run("cover exactly minimum accepted", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(1200, 400), ImageTypeCover)
		assert.NoError(t, err)
	})

	t.Run("post type has no dimension requirement", func(t *testing.T) {
		err := svc.ValidateImageDimensions(makeImg(1, 1), ImageTypePost)
		assert.NoError(t, err)
	})
}

// --- GetImageTypeFromString ---

func TestGetImageTypeFromString(t *testing.T) {
	tests := []struct {
		input   string
		want    ImageType
		wantErr bool
	}{
		{"avatar", ImageTypeAvatar, false},
		{"cover", ImageTypeCover, false},
		{"post", ImageTypePost, false},
		{"AVATAR", ImageTypeAvatar, false},
		{"Cover", ImageTypeCover, false},
		{"video", "", true},
		{"", "", true},
		{"invalid", "", true},
	}

	for _, tc := range tests {
		got, err := GetImageTypeFromString(tc.input)
		if tc.wantErr {
			assert.Error(t, err, "input=%q", tc.input)
		} else {
			assert.NoError(t, err, "input=%q", tc.input)
			assert.Equal(t, tc.want, got, "input=%q", tc.input)
		}
	}
}

// --- mimeFromFormat ---

func TestMimeFromFormat(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"jpeg", "image/jpeg"},
		{"jpg", "image/jpeg"},
		{"JPEG", "image/jpeg"},
		{"png", "image/png"},
		{"gif", "image/gif"},
		{"webp", "image/webp"},
		{"bmp", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tc := range tests {
		got := mimeFromFormat(tc.format)
		assert.Equal(t, tc.want, got, "format=%q", tc.format)
	}
}

// --- isVideoContentType ---

func TestIsVideoContentType(t *testing.T) {
	assert.True(t, isVideoContentType("video/mp4"))
	assert.True(t, isVideoContentType("video/webm"))
	assert.True(t, isVideoContentType("video/quicktime"))
	assert.True(t, isVideoContentType("video/mp4; codecs=avc1"))
	assert.False(t, isVideoContentType("image/jpeg"))
	assert.False(t, isVideoContentType("application/octet-stream"))
	assert.False(t, isVideoContentType(""))
}

// --- mock URL format ---

func TestStorageService_MockURLFormat(t *testing.T) {
	svc := newTestStorageService()
	ctx := context.Background()
	data := makeJPEG(t, 200, 200)
	photo, err := svc.UploadImage(ctx, makeTestFile(data),
		makeHeader("test.jpg", "image/jpeg", int64(len(data))), ImageTypeAvatar)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(photo.URL, "https://storage.hamsaya.local/uploads/"))
}

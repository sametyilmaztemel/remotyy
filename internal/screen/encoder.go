package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
)

// JPEGEncodeOptions configures JPEG encoding.
type JPEGEncodeOptions struct {
	Quality    int  // JPEG quality (1-100), default 80
	Progressive bool // Progressive JPEG, default false
}

// DefaultJPEGOptions returns sensible defaults for JPEG encoding.
func DefaultJPEGOptions() JPEGEncodeOptions {
	return JPEGEncodeOptions{
		Quality: 80,
	}
}

// EncodeJPEG encodes an *image.RGBA to JPEG bytes with the given quality.
// quality should be in range 1-100 (higher = better quality, larger file).
func EncodeJPEG(img *image.RGBA, quality int) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("cannot encode nil image")
	}
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: quality,
	})
	if err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

// EncodeJPEGOpts encodes an image to JPEG with full options.
func EncodeJPEGOpts(img *image.RGBA, opts JPEGEncodeOptions) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("cannot encode nil image")
	}
	if opts.Quality <= 0 {
		opts.Quality = 80
	}
	if opts.Quality > 100 {
		opts.Quality = 100
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{
		Quality: opts.Quality,
	})
	if err != nil {
		return nil, fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

// EncodePNG encodes an *image.RGBA to PNG bytes.
func EncodePNG(img *image.RGBA) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("cannot encode nil image")
	}

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

// EstimateJPEGSize estimates the expected JPEG size for a given image based
// on dimensions and quality. Useful for bandwidth planning.
func EstimateJPEGSize(width, height, quality int) int {
	// Rough heuristic: uncompressed RGBA / compression ratio
	// A quality-80 JPEG compresses roughly 10:1 to 20:1 for photos,
	// 5:1 to 10:1 for UI/screen content.
	pixels := width * height
	rawBytes := pixels * 4 // RGBA

	compressionRatio := 15
	if quality > 90 {
		compressionRatio = 8
	} else if quality > 70 {
		compressionRatio = 12
	} else if quality > 50 {
		compressionRatio = 18
	} else {
		compressionRatio = 25
	}

	estimated := rawBytes / compressionRatio

	// JPEG headers and overhead (~1KB)
	estimated += 1024

	return estimated
}

// compile-time interface check
var _ image.Image = (*image.RGBA)(nil)

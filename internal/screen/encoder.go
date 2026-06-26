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
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
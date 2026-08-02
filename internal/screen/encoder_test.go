package screen

import (
	"image"
	"testing"
)

func TestEncodeJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))

	data, err := EncodeJPEG(img, 80)
	if err != nil {
		t.Fatalf("EncodeJPEG: %v", err)
	}
	if len(data) == 0 {
		t.Error("JPEG data should not be empty")
	}
}

func TestEncodeJPEGNilImage(t *testing.T) {
	_, err := EncodeJPEG(nil, 80)
	if err == nil {
		t.Error("nil image should return error")
	}
}

func TestEncodeJPEGQualityBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Quality 0 → clamped to 1
	data, err := EncodeJPEG(img, 0)
	if err != nil {
		t.Fatalf("EncodeJPEG q=0: %v", err)
	}
	if len(data) == 0 {
		t.Error("should produce output even with low quality")
	}

	// Quality 200 → clamped to 100
	data, err = EncodeJPEG(img, 200)
	if err != nil {
		t.Fatalf("EncodeJPEG q=200: %v", err)
	}
	if len(data) == 0 {
		t.Error("should produce output with high quality")
	}
}

func TestEncodeJPEGOpts(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))

	data, err := EncodeJPEGOpts(img, JPEGEncodeOptions{Quality: 60})
	if err != nil {
		t.Fatalf("EncodeJPEGOpts: %v", err)
	}
	if len(data) == 0 {
		t.Error("JPEG data should not be empty")
	}
}

func TestEncodePNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))

	data, err := EncodePNG(img)
	if err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}
	if len(data) == 0 {
		t.Error("PNG data should not be empty")
	}
}

func TestEncodePNGNilImage(t *testing.T) {
	_, err := EncodePNG(nil)
	if err == nil {
		t.Error("nil image should return error")
	}
}

func TestEstimateJPEGSize(t *testing.T) {
	tests := []struct {
		name    string
		w, h, q int
		minSize int
		maxSize int
	}{
		{"hd_q80", 1920, 1080, 80, 50000, 1000000},
		{"small_q50", 320, 240, 50, 5000, 200000},
		{"4k_q90", 3840, 2160, 90, 100000, 5000000},
		{"tiny_q20", 10, 10, 20, 100, 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := EstimateJPEGSize(tt.w, tt.h, tt.q)
			if size < tt.minSize {
				t.Errorf("estimate too small: %d < %d", size, tt.minSize)
			}
			if size > tt.maxSize {
				t.Errorf("estimate too large: %d > %d", size, tt.maxSize)
			}
		})
	}
}

func TestDefaultJPEGOptions(t *testing.T) {
	opts := DefaultJPEGOptions()
	if opts.Quality != 80 {
		t.Errorf("default quality = %d, want 80", opts.Quality)
	}
}

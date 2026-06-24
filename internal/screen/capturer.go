// Package screen provides screen capture capabilities for remote screen sharing.
package screen

import (
	"fmt"
	"image"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
)

// Capturer captures screenshots for remote viewing.
type Capturer struct {
	cfg        Config
	frameCh    chan *image.RGBA
	stopCh     chan struct{}
	running    bool
}

// Config for screen capturer.
type Config struct {
	FPS          int
	Quality      int
	MaxDimension int
	CaptureCursor bool
	DisplayID    int // 0 = main display
}

// Frame represents a captured screen frame.
type Frame struct {
	Data     []byte // JPEG/WebP encoded
	Width    int
	Height   int
	CapturedAt time.Time
	Duration time.Duration
}

// NewCapturer creates a screen capturer for the current platform.
func NewCapturer(cfg Config) (*Capturer, error) {
	if cfg.FPS == 0 {
		cfg.FPS = 15
	}
	if cfg.Quality == 0 {
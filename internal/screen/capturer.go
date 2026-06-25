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
		cfg.Quality = 60
	}
	if cfg.MaxDimension == 0 {
		cfg.MaxDimension = 1920
	}

	c := &Capturer{
		cfg:     cfg,
		frameCh: make(chan *image.RGBA, 2),
		stopCh:  make(chan struct{}),
	}

	return c, nil
}

// Start begins capturing frames.
func (c *Capturer) Start() error {
	if c.running {
		return nil
	}
	c.running = true

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(c.cfg.FPS))
		defer ticker.Stop()
		defer func() { c.running = false }()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				frame, err := c.captureFrame()
				if err != nil {
					log.Warn().Err(err).Msg("Screen capture failed")
					continue
				}
				if frame != nil {
					select {
					case c.frameCh <- frame:
					default:
						// Drop frame if channel full
					}
				}
			}
		}
	}()

	log.Info().Int("fps", c.cfg.FPS).Msg("Screen capture started")
	return nil
}

// Stop stops capturing.
func (c *Capturer) Stop() {
	if c.running {
		close(c.stopCh)
		c.running = false
	}
	log.Info().Msg("Screen capture stopped")
}

// Frames returns the frame channel.
func (c *Capturer) Frames() <-chan *image.RGBA {
	return c.frameCh
}

// captureFrame captures a single screen frame.
// Platform-specific implementation.
func (c *Capturer) captureFrame() (*image.RGBA, error) {
	switch runtime.GOOS {
	case "darwin":
		return c.captureMacOS()
	case "linux":
		return c.captureLinux()
	default:
		return nil, fmt.Errorf("screen capture not supported on %s", runtime.GOOS)
	}
}

func (c *Capturer) captureMacOS() (*image.RGBA, error) {
	return captureDisplay(c.cfg.DisplayID)
}

func (c *Capturer) captureLinux() (*image.RGBA, error) {
	// Linux: Use X11 or PipeWire
	return nil, fmt.Errorf("Linux screen capture requires X11/PipeWire")
}

// EncodeJPEG is implemented in encoder.go (cross-platform file).

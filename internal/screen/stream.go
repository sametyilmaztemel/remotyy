package screen

import (
	"fmt"
	"image"
	"image/color"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// FrameCallback is called for each captured frame.
// The callback receives encoded JPEG data, width, height, and the capture duration.
// If the callback returns false, the capture loop stops.
type FrameCallback func(data []byte, width, height int, capturedAt time.Time, duration time.Duration) bool

// StreamConfig configures the screen streaming session.
type StreamConfig struct {
	FPS           int   // Target frames per second (default: 15)
	Quality       int   // JPEG quality 1-100 (default: 60)
	DisplayID     int   // Display to capture (0 = main display)
	MaxWidth      int   // Maximum width for downscaling (0 = no scaling)
	MaxHeight     int   // Maximum height for downscaling (0 = no scaling)
	AdaptiveQuality bool // Whether to dynamically adjust quality based on frame timing
	MinQuality    int   // Minimum quality when adaptive is enabled (default: 20)
	MaxQuality    int   // Maximum quality when adaptive is enabled (default: 90)
}

// DefaultStreamConfig returns a StreamConfig with sensible defaults.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		FPS:            15,
		Quality:        60,
		DisplayID:      0,
		MaxWidth:       0,
		MaxHeight:      0,
		AdaptiveQuality: true,
		MinQuality:     20,
		MaxQuality:     90,
	}
}

// Streamer manages a screen capture loop that captures frames and sends
// them to a callback. It supports adaptive quality to maintain target FPS.
type Streamer struct {
	cfg       StreamConfig
	mu        sync.Mutex
	stopCh    chan struct{}
	running   bool
	callback  FrameCallback

	// Adaptive quality tracking
	lastFrameTime time.Time
	frameCount    int
	frameTimes    []time.Duration
	avgFrameDur   time.Duration

	// Stats
	framesCaptured int64
	framesSkipped  int64
	totalBytes     int64
	startTime      time.Time
}

// NewStreamer creates a new screen Streamer with the given config and callback.
func NewStreamer(cfg StreamConfig, callback FrameCallback) (*Streamer, error) {
	if callback == nil {
		return nil, fmt.Errorf("frame callback cannot be nil")
	}
	if cfg.FPS <= 0 {
		cfg.FPS = 15
	}
	if cfg.Quality <= 0 {
		cfg.Quality = 60
	}
	if cfg.MinQuality <= 0 {
		cfg.MinQuality = 20
	}
	if cfg.MaxQuality <= 0 {
		cfg.MaxQuality = 90
	}
	if cfg.MaxQuality < cfg.MinQuality {
		cfg.MaxQuality = cfg.MinQuality + 10
	}
	if cfg.Quality < cfg.MinQuality {
		cfg.Quality = cfg.MinQuality
	}
	if cfg.Quality > cfg.MaxQuality {
		cfg.Quality = cfg.MaxQuality
	}

	s := &Streamer{
		cfg:       cfg,
		stopCh:    make(chan struct{}),
		callback:  callback,
		frameTimes: make([]time.Duration, 0, 30),
	}

	return s, nil
}

// Start begins the capture loop. This blocks until Stop is called or
// the callback returns false. To run non-blocking, call in a goroutine.
func (s *Streamer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.startTime = time.Now()
	s.lastFrameTime = time.Now()
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Info().
		Int("fps", s.cfg.FPS).
		Int("quality", s.cfg.Quality).
		Int("display", s.cfg.DisplayID).
		Bool("adaptive", s.cfg.AdaptiveQuality).
		Msg("Screen streamer started")

	frameInterval := time.Second / time.Duration(s.cfg.FPS)
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Info().Msg("Screen streamer stopped")
			return nil
		case <-ticker.C:
			continueRun := s.captureAndSend()
			if !continueRun {
				log.Info().Msg("Screen streamer stopped by callback")
				return nil
			}
		}
	}
}

// StartAsync starts the capture loop in a background goroutine.
// Returns any immediate errors. Use Stop() to terminate the loop.
func (s *Streamer) StartAsync() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.startTime = time.Now()
	s.lastFrameTime = time.Now()
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go func() {
		log.Info().
			Int("fps", s.cfg.FPS).
			Int("quality", s.cfg.Quality).
			Int("display", s.cfg.DisplayID).
			Bool("adaptive", s.cfg.AdaptiveQuality).
			Msg("Screen streamer started (async)")

		frameInterval := time.Second / time.Duration(s.cfg.FPS)
		ticker := time.NewTicker(frameInterval)
		defer ticker.Stop()
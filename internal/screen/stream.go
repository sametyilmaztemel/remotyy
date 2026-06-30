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

		for {
			select {
			case <-s.stopCh:
				log.Info().Msg("Screen streamer stopped")
				return
			case <-ticker.C:
				continueRun := s.captureAndSend()
				if !continueRun {
					s.mu.Lock()
					s.running = false
					s.mu.Unlock()
					log.Info().Msg("Screen streamer stopped by callback")
					return
				}
			}
		}
	}()

	return nil
}

// Stop signals the capture loop to stop.
func (s *Streamer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// IsRunning returns whether the streamer is currently running.
func (s *Streamer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Stats returns current streaming statistics.
func (s *Streamer) Stats() StreamStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	uptime := time.Since(s.startTime)
	fps := 0.0
	if uptime.Seconds() > 0 {
		fps = float64(s.framesCaptured) / uptime.Seconds()
	}

	currentQuality := s.cfg.Quality
	if s.cfg.AdaptiveQuality {
		currentQuality = s.computeAdaptiveQuality()
	}

	return StreamStats{
		FramesCaptured: s.framesCaptured,
		FramesSkipped:  s.framesSkipped,
		TotalBytes:     s.totalBytes,
		Uptime:         uptime,
		AverageFPS:     fps,
		CurrentQuality: currentQuality,
		IsRunning:      s.running,
	}
}

// UpdateConfig allows changing streaming parameters on the fly.
// DisplayID and MaxWidth/MaxHeight changes take effect on the next frame.
func (s *Streamer) UpdateConfig(cfg StreamConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.FPS > 0 {
		s.cfg.FPS = cfg.FPS
	}
	if cfg.Quality > 0 {
		s.cfg.Quality = cfg.Quality
	}
	if cfg.MinQuality > 0 {
		s.cfg.MinQuality = cfg.MinQuality
	}
	if cfg.MaxQuality > 0 {
		s.cfg.MaxQuality = cfg.MaxQuality
	}
	s.cfg.DisplayID = cfg.DisplayID
	s.cfg.MaxWidth = cfg.MaxWidth
	s.cfg.MaxHeight = cfg.MaxHeight
	s.cfg.AdaptiveQuality = cfg.AdaptiveQuality
}

// captureAndSend captures a single frame, encodes it, and sends it
// via the callback. Returns true to continue, false to stop.
func (s *Streamer) captureAndSend() bool {
	startCapture := time.Now()

	// Capture the display
	rgba, err := captureDisplay(s.cfg.DisplayID)
	if err != nil {
		log.Warn().Err(err).Msg("Screen capture failed")
		s.mu.Lock()
		s.framesSkipped++
		s.mu.Unlock()
		return true // continue despite error
	}

	// Downscale if configured
	if s.cfg.MaxWidth > 0 || s.cfg.MaxHeight > 0 {
		rgba = s.downscale(rgba)
	}

	// Determine quality (adaptive or fixed)
	quality := s.cfg.Quality
	if s.cfg.AdaptiveQuality {
		s.mu.Lock()
		quality = s.computeAdaptiveQuality()
		s.mu.Unlock()
	}

	// Encode to JPEG
	encoded, err := EncodeJPEG(rgba, quality)
	if err != nil {
		log.Warn().Err(err).Msg("JPEG encoding failed")
		s.mu.Lock()
		s.framesSkipped++
		s.mu.Unlock()
		return true
	}

	captureDuration := time.Since(startCapture)
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Track timing for adaptive quality
	s.mu.Lock()
	s.framesCaptured++
	s.totalBytes += int64(len(encoded))
	now := time.Now()
	elapsed := now.Sub(s.lastFrameTime)
	s.lastFrameTime = now

	// Keep rolling average of last N frame durations
	s.frameTimes = append(s.frameTimes, elapsed)
	if len(s.frameTimes) > 30 {
		s.frameTimes = s.frameTimes[1:]
	}
	s.mu.Unlock()

	// Call the callback with the encoded frame
	return s.callback(encoded, width, height, startCapture, captureDuration)
}

// computeAdaptiveQuality adjusts JPEG quality based on recent frame timing
// to maintain the target FPS. If frames are taking too long, quality decreases;
// if there's slack, quality increases.
func (s *Streamer) computeAdaptiveQuality() int {
	if !s.cfg.AdaptiveQuality || len(s.frameTimes) == 0 {
		return s.cfg.Quality
	}

	// Compute rolling average frame duration
	var total time.Duration
	for _, d := range s.frameTimes {
		total += d
	}
	s.avgFrameDur = total / time.Duration(len(s.frameTimes))
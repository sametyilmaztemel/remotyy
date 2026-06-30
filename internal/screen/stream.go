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

	// Target frame interval
	targetInterval := time.Second / time.Duration(s.cfg.FPS)

	// How much of the frame budget are we using?
	ratio := float64(s.avgFrameDur) / float64(targetInterval)

	currentQuality := s.cfg.Quality

	switch {
	case ratio > 0.85:
		// Running out of time — reduce quality
		currentQuality -= 5
	case ratio < 0.4 && currentQuality < s.cfg.MaxQuality:
		// Lots of slack — increase quality
		currentQuality += 5
	case ratio < 0.6 && currentQuality < s.cfg.MaxQuality:
		// Some slack — increase quality slowly
		currentQuality += 2
	}

	// Clamp
	if currentQuality < s.cfg.MinQuality {
		currentQuality = s.cfg.MinQuality
	}
	if currentQuality > s.cfg.MaxQuality {
		currentQuality = s.cfg.MaxQuality
	}

	s.cfg.Quality = currentQuality
	return currentQuality
}

// downscale performs bilinear downscaling of the image to fit within
// MaxWidth x MaxHeight while maintaining aspect ratio.
func (s *Streamer) downscale(src *image.RGBA) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	maxW := s.cfg.MaxWidth
	maxH := s.cfg.MaxHeight

	// If no max or image is already smaller, return as-is
	if (maxW <= 0 || srcW <= maxW) && (maxH <= 0 || srcH <= maxH) {
		return src
	}

	// If one dimension is unconstrained, use the other
	if maxW <= 0 {
		maxW = srcW
	}
	if maxH <= 0 {
		maxH = srcH
	}

	// Calculate scaled dimensions maintaining aspect ratio
	dstW, dstH := srcW, srcH
	if srcW > maxW {
		dstW = maxW
		dstH = srcH * maxW / srcW
	}
	if dstH > maxH {
		dstH = maxH
		dstW = srcW * maxH / srcH
	}

	if dstW <= 0 {
		dstW = 1
	}
	if dstH <= 0 {
		dstH = 1
	}

	// Only scale if actually needed
	if dstW == srcW && dstH == srcH {
		return src
	}

	return scaleBilinear(src, dstW, dstH)
}

// scaleBilinear performs a simple bilinear interpolation downscale.
func scaleBilinear(src *image.RGBA, dstW, dstH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// Simple bilinear interpolation
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			// Map destination pixel to source coordinates
			sx := float64(dx) * float64(srcW) / float64(dstW)
			sy := float64(dy) * float64(srcH) / float64(dstH)

			// Get surrounding pixels
			x0 := int(sx)
			y0 := int(sy)
			x1 := x0 + 1
			y1 := y0 + 1

			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			// Fractional parts
			xFrac := sx - float64(x0)
			yFrac := sy - float64(y0)

			// Sample four pixels
			p00 := src.RGBAAt(x0+srcBounds.Min.X, y0+srcBounds.Min.Y)
			p10 := src.RGBAAt(x1+srcBounds.Min.X, y0+srcBounds.Min.Y)
			p01 := src.RGBAAt(x0+srcBounds.Min.X, y1+srcBounds.Min.Y)
			p11 := src.RGBAAt(x1+srcBounds.Min.X, y1+srcBounds.Min.Y)

			// Bilinear interpolate each channel
			r := lerp(lerp(float64(p00.R), float64(p10.R), xFrac),
				lerp(float64(p01.R), float64(p11.R), xFrac), yFrac)
			g := lerp(lerp(float64(p00.G), float64(p10.G), xFrac),
				lerp(float64(p01.G), float64(p11.G), xFrac), yFrac)
			b := lerp(lerp(float64(p00.B), float64(p10.B), xFrac),
				lerp(float64(p01.B), float64(p11.B), xFrac), yFrac)
			a := lerp(lerp(float64(p00.A), float64(p10.A), xFrac),
				lerp(float64(p01.A), float64(p11.A), xFrac), yFrac)

			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8(clamp(int(r+0.5), 0, 255)),
				G: uint8(clamp(int(g+0.5), 0, 255)),
				B: uint8(clamp(int(b+0.5), 0, 255)),
				A: uint8(clamp(int(a+0.5), 0, 255)),
			})
		}
	}

	return dst
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// StreamStats contains streaming statistics.
type StreamStats struct {
	FramesCaptured int64         `json:"frames_captured"`
	FramesSkipped  int64         `json:"frames_skipped"`
	TotalBytes     int64         `json:"total_bytes"`
	Uptime         time.Duration `json:"uptime"`
	AverageFPS     float64       `json:"average_fps"`
	CurrentQuality int           `json:"current_quality"`
	IsRunning      bool          `json:"is_running"`
}

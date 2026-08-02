package screen

import (
	"testing"
	"time"
)

func TestDefaultStreamConfig(t *testing.T) {
	cfg := DefaultStreamConfig()
	if cfg.FPS != 15 {
		t.Errorf("default FPS = %d, want 15", cfg.FPS)
	}
	if cfg.Quality != 60 {
		t.Errorf("default quality = %d, want 60", cfg.Quality)
	}
	if cfg.DisplayID != 0 {
		t.Errorf("default display ID = %d, want 0", cfg.DisplayID)
	}
	if !cfg.AdaptiveQuality {
		t.Error("adaptive quality should be enabled by default")
	}
}

func TestNewStreamerNilCallback(t *testing.T) {
	_, err := NewStreamer(DefaultStreamConfig(), nil)
	if err == nil {
		t.Error("nil callback should return error")
	}
}

func TestNewStreamerInvalidFPS(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.FPS = 0
	s, err := NewStreamer(cfg, func([]byte, int, int, time.Time, time.Duration) bool {
		return true
	})
	if err != nil {
		t.Fatalf("NewStreamer: %v", err)
	}
	if s.cfg.FPS != 15 {
		t.Errorf("FPS should default to 15, got %d", s.cfg.FPS)
	}
}

func TestNewStreamerQualityBounds(t *testing.T) {
	tests := []struct {
		name     string
		quality  int
		minQ     int
		maxQ     int
		want     int
	}{
		{"default", 60, 20, 90, 60},
		{"clamp_below_min", 5, 20, 90, 20},
		{"clamp_above_max", 95, 20, 90, 90},
		{"min_equals_max", 50, 50, 50, 50},
		{"min_above_max", 50, 80, 60, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultStreamConfig()
			cfg.Quality = tt.quality
			cfg.MinQuality = tt.minQ
			cfg.MaxQuality = tt.maxQ
			s, err := NewStreamer(cfg, func([]byte, int, int, time.Time, time.Duration) bool {
				return true
			})
			if err != nil {
				t.Fatalf("NewStreamer: %v", err)
			}
			if s.cfg.Quality != tt.want {
				t.Errorf("quality = %d, want %d", s.cfg.Quality, tt.want)
			}
		})
	}
}

func TestStreamerStartStop(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.FPS = 60 // faster frames for test

	s, err := NewStreamer(cfg, func([]byte, int, int, time.Time, time.Duration) bool {
		return false // stop immediately on first frame
	})
	if err != nil {
		t.Fatalf("NewStreamer: %v", err)
	}

	if s.IsRunning() {
		t.Error("should not be running before Start")
	}

	// StartAsync to avoid blocking on non-macOS (capture always fails)
	err = s.StartAsync()
	if err != nil {
		t.Fatalf("StartAsync: %v", err)
	}

	if !s.IsRunning() {
		t.Error("should be running after StartAsync")
	}

	// Give it a moment to attempt capture
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	if s.IsRunning() {
		t.Error("should not be running after Stop")
	}

	// Second stop should be safe
	s.Stop()
}

func TestStreamerStartAsyncStop(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.FPS = 60

	called := make(chan struct{})
	s, err := NewStreamer(cfg, func([]byte, int, int, time.Time, time.Duration) bool {
		select {
		case called <- struct{}{}:
		default:
		}
		return true
	})
	if err != nil {
		t.Fatalf("NewStreamer: %v", err)
	}

	err = s.StartAsync()
	if err != nil {
		t.Fatalf("StartAsync: %v", err)
	}

	if !s.IsRunning() {
		t.Error("should be running after StartAsync")
	}

	// Wait for at least one capture attempt
	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Log("No capture call received (expected on non-macOS)")
	}

	s.Stop()

	if s.IsRunning() {
		t.Error("should not be running after Stop")
	}

	// Second stop should be safe
	s.Stop()
}

func TestStreamerStats(t *testing.T) {
	cfg := DefaultStreamConfig()
	s, err := NewStreamer(cfg, func([]byte, int, int, time.Time, time.Duration) bool {
		return false
	})
	if err != nil {
		t.Fatalf("NewStreamer: %v", err)
	}

	stats := s.Stats()
	if stats.IsRunning {
		t.Error("should not be running before start")
	}
	if stats.FramesCaptured != 0 {
		t.Errorf("frames captured = %d, want 0", stats.FramesCaptured)
	}
}

func TestStreamerUpdateConfig(t *testing.T) {
	s, err := NewStreamer(DefaultStreamConfig(), func([]byte, int, int, time.Time, time.Duration) bool {
		return true
	})
	if err != nil {
		t.Fatalf("NewStreamer: %v", err)
	}

	// Update config while not running
	newCfg := DefaultStreamConfig()
	newCfg.FPS = 30
	newCfg.Quality = 80
	s.UpdateConfig(newCfg)

	if s.cfg.FPS != 30 {
		t.Errorf("FPS = %d, want 30", s.cfg.FPS)
	}
	if s.cfg.Quality != 80 {
		t.Errorf("quality = %d, want 80", s.cfg.Quality)
	}
}

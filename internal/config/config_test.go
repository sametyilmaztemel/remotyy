package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load default config: %v", err)
	}

	if cfg.Signal.Host != DefaultSignalHost {
		t.Errorf("Signal.Host = %q, want %q", cfg.Signal.Host, DefaultSignalHost)
	}
	if cfg.Signal.Port != DefaultSignalPort {
		t.Errorf("Signal.Port = %d, want %d", cfg.Signal.Port, DefaultSignalPort)
	}
	if cfg.Logging.Level != DefaultLogLevel {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, DefaultLogLevel)
	}
	if cfg.Screen.FPS != 15 {
		t.Errorf("Screen.FPS = %d, want 15", cfg.Screen.FPS)
	}
	if cfg.Screen.Quality != 60 {
		t.Errorf("Screen.Quality = %d, want 60", cfg.Screen.Quality)
	}
	if cfg.Screen.MaxDimension != 1920 {
		t.Errorf("Screen.MaxDimension = %d, want 1920", cfg.Screen.MaxDimension)
	}
}

func TestSignalAddr(t *testing.T) {
	cfg := SignalConfig{Host: "127.0.0.1", Port: 9000}
	if cfg.Addr() != "127.0.0.1:9000" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), "127.0.0.1:9000")
	}
}

func TestWSSAddr(t *testing.T) {
	tests := []struct {
		name string
		cfg  SignalConfig
		want string
	}{
		{
			name: "ws no TLS",
			cfg:  SignalConfig{Host: "0.0.0.0", Port: 9000, TLS: TLSConfig{Enabled: false}},
			want: "ws://0.0.0.0:9000",
		},
		{
			name: "wss with TLS",
			cfg:  SignalConfig{Host: "example.com", Port: 443, TLS: TLSConfig{Enabled: true}},
			want: "wss://example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.WSSAddr(); got != tt.want {
				t.Errorf("WSSAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"error", true},
		{"DEBUG", true},
		{"INFO", true},
		{"unknown", true}, // defaults to info
	}

	cfg := LoggingConfig{}
	for _, tt := range tests {
		cfg.Level = tt.input
		level := cfg.ParseLevel()
		if level.String() == "" {
			t.Errorf("ParseLevel(%q) returned empty", tt.input)
		}
	}
}

func TestConfigFromYAML(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "remotty.yaml")
	content := []byte(`
signal:
  host: "192.168.1.1"
  port: 8080
  dev_mode: true
  rate_limit: 120

host:
  name: "test-host"
  reconnect_wait: 10s
  heartbeat_interval: 30s

logging:
  level: "debug"
  format: "json"

screen:
  enabled: true
  fps: 30
  quality: 80
  max_dimension: 1280
`)

	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load YAML: %v", err)
	}

	if cfg.Signal.Host != "192.168.1.1" {
		t.Errorf("Signal.Host = %q, want %q", cfg.Signal.Host, "192.168.1.1")
	}
	if cfg.Signal.Port != 8080 {
		t.Errorf("Signal.Port = %d, want 8080", cfg.Signal.Port)
	}
	if !cfg.Signal.DevMode {
		t.Error("Signal.DevMode = false, want true")
	}
	if cfg.Signal.RateLimit != 120 {
		t.Errorf("Signal.RateLimit = %d, want 120", cfg.Signal.RateLimit)
	}
	if cfg.Host.Name != "test-host" {
		t.Errorf("Host.Name = %q, want %q", cfg.Host.Name, "test-host")
	}
	if cfg.Host.ReconnectWait != 10*time.Second {
		t.Errorf("Host.ReconnectWait = %v, want 10s", cfg.Host.ReconnectWait)
	}
	if cfg.Host.HeartbeatInt != 30*time.Second {
		t.Errorf("Host.HeartbeatInt = %v, want 30s", cfg.Host.HeartbeatInt)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
	if !cfg.Screen.Enabled {
		t.Error("Screen.Enabled = false, want true")
	}
	if cfg.Screen.FPS != 30 {
		t.Errorf("Screen.FPS = %d, want 30", cfg.Screen.FPS)
	}
	if cfg.Screen.Quality != 80 {
		t.Errorf("Screen.Quality = %d, want 80", cfg.Screen.Quality)
	}
	if cfg.Screen.MaxDimension != 1280 {
		t.Errorf("Screen.MaxDimension = %d, want 1280", cfg.Screen.MaxDimension)
	}
}

func TestConfigFromEnv(t *testing.T) {
	os.Setenv("REMOTTY_AUTH_TOKEN", "test-token-123")
	os.Setenv("REMOTTY_LOG_LEVEL", "warn")
	defer os.Unsetenv("REMOTTY_AUTH_TOKEN")
	defer os.Unsetenv("REMOTTY_LOG_LEVEL")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}

	if cfg.Signal.AuthToken != "test-token-123" {
		t.Errorf("Signal.AuthToken = %q, want %q", cfg.Signal.AuthToken, "test-token-123")
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "warn")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that host name falls back to hostname
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	hostname, _ := os.Hostname()
	if cfg.Host.Name != hostname {
		t.Errorf("Host.Name = %q, want %q (hostname fallback)", cfg.Host.Name, hostname)
	}
}

func TestDefaultDataDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".remotty")
	dir := defaultDataDir()
	if dir != expected {
		t.Errorf("defaultDataDir() = %q, want %q", dir, expected)
	}
}

func TestValidate(t *testing.T) {
	// Empty config should fail with new stricter validation
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("empty config should fail validation")
	}
}

func TestVersionDefaults(t *testing.T) {
	if Version != "dev" && Version == "" {
		t.Error("Version should default to 'dev'")
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000, DevMode: true},
		Host:    HostConfig{ReconnectWait: 5 * time.Second, HeartbeatInt: 15 * time.Second, SessionTimeout: 30 * time.Minute, MaxSessions: 10},
		WebRTC:  WebRTCConfig{ICETimeout: 10, MaxMessageSize: 65536},
		Screen:  ScreenConfig{FPS: 15, Quality: 60, MaxDimension: 1920},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 99999},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("invalid port should fail validation")
	}
}

func TestValidateTLSMissingCert(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000, TLS: TLSConfig{Enabled: true}},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("TLS without cert should fail validation")
	}
}

func TestValidateInvalidLogLevel(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000},
		Logging: LoggingConfig{Level: "verbose"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("invalid log level should fail validation")
	}
}

func TestValidateScreenOutOfRange(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000},
		Screen:  ScreenConfig{FPS: 200, Quality: 60, MaxDimension: 1920},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("FPS > 120 should fail validation")
	}
}

func TestValidateRequireAuthWithoutPassword(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000},
		Host:    HostConfig{RequireAuth: true},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("require_auth without password should fail validation")
	}
}

func TestValidateRequireAuthWithPassword(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000, DevMode: true},
		Host:    HostConfig{RequireAuth: true, MasterHash: "$2a$10$somehash", Name: "test-host"},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("require_auth with hash should pass: %v", err)
	}
}

func TestValidateNegativeValues(t *testing.T) {
	cfg := Config{
		Signal:  SignalConfig{Port: 9000},
		Host:    HostConfig{ReconnectWait: -1 * time.Second},
		WebRTC:  WebRTCConfig{ICETimeout: -5},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("negative values should fail validation")
	}
}

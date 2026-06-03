// Package config provides centralized configuration for all remotty components.
// Supports loading from YAML file, environment variables, and CLI flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// Defaults
const (
	DefaultSignalPort     = 9000
	DefaultSignalHost     = "0.0.0.0"
	DefaultWebPort        = 3000
	DefaultSTUNServer     = "stun:stun.l.google.com:19302"
	DefaultReconnectWait  = 5 * time.Second
	DefaultReconnectMaxWait = 60 * time.Second
	DefaultHeartbeatInt   = 15 * time.Second
	DefaultSessionTimeout = 30 * time.Minute
	DefaultMaxSessions    = 10
	DefaultLogLevel       = "info"
)

// Config is the top-level configuration.
type Config struct {
	Global   GlobalConfig   `mapstructure:"global"`
	Signal   SignalConfig   `mapstructure:"signal"`
	Host     HostConfig     `mapstructure:"host"`
	Client   ClientConfig   `mapstructure:"client"`
	WebRTC   WebRTCConfig   `mapstructure:"webrtc"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Screen   ScreenConfig   `mapstructure:"screen"`
}

// GlobalConfig contains global settings.
type GlobalConfig struct {
	DataDir    string `mapstructure:"data_dir"`
	ConfigFile string `mapstructure:"config_file"`
}

// SignalConfig for the signaling server.
type SignalConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	TLS             TLSConfig     `mapstructure:"tls"`
	AuthToken       string        `mapstructure:"auth_token"`
	RateLimit       int           `mapstructure:"rate_limit"`
	AllowedOrigins  []string      `mapstructure:"allowed_origins"`
	DevMode         bool          `mapstructure:"dev_mode"`
	WebDir          string        `mapstructure:"web_dir"`
}

// TLSConfig for encrypted connections.
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// HostConfig for the host daemon.
type HostConfig struct {
	SignalURL       string        `mapstructure:"signal_url"`
	Name            string        `mapstructure:"name"`
	MasterPassword  string        `mapstructure:"master_password"`
	MasterHash      string        `mapstructure:"master_hash"`
	AllowList       []string      `mapstructure:"allow_list"`
	Features        []string      `mapstructure:"features"`
	ReconnectWait   time.Duration `mapstructure:"reconnect_wait"`
	ReconnectMaxWait time.Duration `mapstructure:"reconnect_max_wait"`
	HeartbeatInt    time.Duration `mapstructure:"heartbeat_interval"`
	SessionTimeout  time.Duration `mapstructure:"session_timeout"`
	MaxSessions     int           `mapstructure:"max_sessions"`
	RequireAuth     bool          `mapstructure:"require_auth"`
	DeviceID        string        `mapstructure:"device_id"`
	ShowQR          bool          `mapstructure:"show_qr"`
	OnRegistered    func(peerID string)
}

// ClientConfig for the client.
type ClientConfig struct {
	SignalURL      string `mapstructure:"signal_url"`
	HostID         string `mapstructure:"host_id"`
	MasterPassword string `mapstructure:"master_password"`
	Insecure       bool   `mapstructure:"insecure"`
}

// WebRTCConfig for ICE and peer connections.
type WebRTCConfig struct {
	ICEServers    []string `mapstructure:"ice_servers"`
	MDNS          bool     `mapstructure:"mdns"`
	ICETimeout    int      `mapstructure:"ice_timeout"`
	MaxMessageSize int     `mapstructure:"max_message_size"`
}

// LoggingConfig for output control.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json, console
	File   string `mapstructure:"file"`
}

// ScreenConfig for screen sharing.
type ScreenConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	FPS          int     `mapstructure:"fps"`
	Quality      int     `mapstructure:"quality"`
	MaxDimension int     `mapstructure:"max_dimension"`
	CaptureCursor bool   `mapstructure:"capture_cursor"`
}

// Load reads configuration from file, env, and defaults.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("global.data_dir", defaultDataDir())
	v.SetDefault("signal.host", DefaultSignalHost)
	v.SetDefault("signal.port", DefaultSignalPort)
	v.SetDefault("signal.rate_limit", 60)
	v.SetDefault("signal.dev_mode", false)
	v.SetDefault("host.reconnect_wait", DefaultReconnectWait)
	v.SetDefault("host.reconnect_max_wait", DefaultReconnectMaxWait)
	v.SetDefault("host.heartbeat_interval", DefaultHeartbeatInt)
	v.SetDefault("host.session_timeout", DefaultSessionTimeout)
	v.SetDefault("host.max_sessions", 0) // 0 = unlimited
	v.SetDefault("host.features", []string{"terminal"})
package host

import (
	"runtime"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/config"
)

func TestNewDaemonDefaults(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name: "test-host",
	}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	if d.cfg.Name != "test-host" {
		t.Errorf("name = %q, want test-host", d.cfg.Name)
	}
	if d.cfg.MaxSessions != 10 {
		t.Errorf("max sessions = %d, want 10", d.cfg.MaxSessions)
	}
	if d.cfg.ReconnectWait == 0 {
		t.Error("reconnect wait should default")
	}
	if d.cfg.HeartbeatInt == 0 {
		t.Error("heartbeat interval should default")
	}
}

func TestNewDaemonNoAuthWarning(t *testing.T) {
	// Should succeed but warn (no password)
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name:           "no-auth-host",
		MasterPassword: "",
		MasterHash:     "",
	}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon without auth should succeed: %v", err)
	}
	if d == nil {
		t.Error("daemon should not be nil")
	}
}

func TestNewDaemonRequireAuthWithoutPassword(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name:        "require-auth-host",
		RequireAuth: true,
	}

	_, err := NewDaemon(cfg, log)
	if err == nil {
		t.Error("NewDaemon with require_auth and no password should fail")
	}
}

func TestNewDaemonRequireAuthWithPassword(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name:           "require-auth-host",
		RequireAuth:    true,
		MasterPassword: "secret123",
	}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon with require_auth and password should succeed: %v", err)
	}
	if d.cfg.MasterHash == "" {
		t.Error("password should be hashed and stored in MasterHash")
	}
}

func TestNewDaemonPasswordHashed(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name:           "hash-test",
		MasterPassword: "mypassword",
	}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if d.cfg.MasterHash == "" {
		t.Error("MasterPassword should be auto-hashed into MasterHash")
	}
	if d.cfg.MasterPassword != "mypassword" {
		t.Error("MasterPassword should be preserved")
	}
}

func TestNewDaemonExistingHash(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{
		Name:       "existing-hash",
		MasterHash: "$2a$10$existinghashvalue",
	}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if d.cfg.MasterHash != "$2a$10$existinghashvalue" {
		t.Error("existing hash should be preserved")
	}
}

func TestNewDaemonHostnameFallback(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{} // No name

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if d.cfg.Name == "" {
		t.Error("name should fall back to hostname")
	}
}

func TestNewDaemonFeaturesDefault(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.HostConfig{Name: "test"}

	d, err := NewDaemon(cfg, log)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// On darwin, the default features include "screen" alongside "terminal"
	expected := []string{"terminal"}
	if runtime.GOOS == "darwin" {
		expected = []string{"terminal", "screen"}
	}

	if len(d.cfg.Features) != len(expected) {
		t.Errorf("features = %v, want %v", d.cfg.Features, expected)
	}
	for i, f := range expected {
		if d.cfg.Features[i] != f {
			t.Errorf("features[%d] = %q, want %q", i, d.cfg.Features[i], f)
		}
	}
}

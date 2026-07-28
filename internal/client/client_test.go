package client

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/config"
)

func TestNewClient(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.ClientConfig{
		SignalURL:      "ws://localhost:9000",
		HostID:         "test-host",
		MasterPassword: "secret",
	}

	c, err := NewClient(cfg, log)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.cfg.HostID != "test-host" {
		t.Errorf("hostID = %q, want test-host", c.cfg.HostID)
	}
	if c.cfg.SignalURL != "ws://localhost:9000" {
		t.Errorf("signalURL = %q, want ws://localhost:9000", c.cfg.SignalURL)
	}
}

func TestNewClientDefaults(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.ClientConfig{}

	c, err := NewClient(cfg, log)
	if err != nil {
		t.Fatalf("NewClient with empty config: %v", err)
	}
	if c == nil {
		t.Fatal("client should not be nil")
	}
	if len(c.hosts) != 0 {
		t.Error("hosts should be empty initially")
	}
}

func TestNewClientInsecureFlag(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.ClientConfig{
		SignalURL: "wss://secure.example.com",
		Insecure:  true,
	}

	c, err := NewClient(cfg, log)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if !c.cfg.Insecure {
		t.Error("insecure flag should be preserved")
	}
}

func TestClientEmptyHostID(t *testing.T) {
	log := zerolog.Nop()
	cfg := config.ClientConfig{
		SignalURL: "ws://localhost:9000",
		HostID:    "",
	}

	c, err := NewClient(cfg, log)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.cfg.HostID != "" {
		t.Error("empty host ID should be preserved as empty")
	}
}

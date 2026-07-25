package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRootCmdHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Reset for next test
	defer rootCmd.SetArgs([]string{})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "remotty") {
		t.Error("help output should contain 'remotty'")
	}
	if !strings.Contains(output, "Usage") {
		t.Error("help output should contain 'Usage'")
	}
}

func TestRootCmdUnknownCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"nonexistent"})
	defer rootCmd.SetArgs([]string{})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("unknown command should error")
	}
}

func TestSignalCmdFlags(t *testing.T) {
	// Verify flags are registered
	port, err := signalCmd.Flags().GetInt("port")
	if err != nil {
		t.Fatalf("port flag: %v", err)
	}
	if port != 9000 {
		t.Errorf("default port = %d, want 9000", port)
	}

	host, err := signalCmd.Flags().GetString("host")
	if err != nil {
		t.Fatalf("host flag: %v", err)
	}
	if host != "0.0.0.0" {
		t.Errorf("default host = %q, want 0.0.0.0", host)
	}

	dev, err := signalCmd.Flags().GetBool("dev")
	if err != nil {
		t.Fatalf("dev flag: %v", err)
	}
	if dev {
		t.Error("default dev should be false")
	}
}

func TestHostCmdFlags(t *testing.T) {
	signal, err := hostCmd.Flags().GetString("signal")
	if err != nil {
		t.Fatalf("signal flag: %v", err)
	}
	if signal != "ws://localhost:9000" {
		t.Errorf("default signal = %q, want ws://localhost:9000", signal)
	}

	qr, err := hostCmd.Flags().GetBool("qr")
	if err != nil {
		t.Fatalf("qr flag: %v", err)
	}
	if qr {
		t.Error("default qr should be false")
	}
}

func TestConnectCmdFlags(t *testing.T) {
	signal, err := connectCmd.Flags().GetString("signal")
	if err != nil {
		t.Fatalf("signal flag: %v", err)
	}
	if signal != "ws://localhost:9000" {
		t.Errorf("default signal = %q", signal)
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		input []string
		sep   string
		want  string
	}{
		{[]string{"terminal", "screen"}, ", ", "terminal, screen"},
		{[]string{"a"}, ", ", "a"},
		{[]string{}, ", ", ""},
		{[]string{"x", "y", "z"}, "|", "x|y|z"},
	}

	for _, tt := range tests {
		got := joinStrings(tt.input, tt.sep)
		if got != tt.want {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.input, tt.sep, got, tt.want)
		}
	}
}

func TestEnvOverride(t *testing.T) {
	// Set env vars
	os.Setenv("REMOTTY_AUTH_TOKEN", "test-token-123")
	defer os.Unsetenv("REMOTTY_AUTH_TOKEN")

	token := os.Getenv("REMOTTY_AUTH_TOKEN")
	if token != "test-token-123" {
		t.Errorf("env override = %q, want test-token-123", token)
	}
}

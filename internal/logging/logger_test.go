package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitConsoleFormat(t *testing.T) {
	logger, err := Init(zerolog.DebugLevel, "console", "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
	if logger.Audit == nil {
		t.Fatal("audit logger should not be nil")
	}
}

func TestInitJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := Init(zerolog.InfoLevel, "json", logFile)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
}

func TestInitInvalidPath(t *testing.T) {
	_, err := Init(zerolog.InfoLevel, "json", "/nonexistent/dir/test.log")
	if err == nil {
		t.Error("expected error for invalid log file path")
	}
}

func TestAuditLoggerDiscard(t *testing.T) {
	// Empty log file = io.Discard
	audit, err := NewAuditLogger("")
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	// Should not panic
	audit.Log("test_event", "peer-1", "room-1", "1.2.3.4", "detail", true)
}

func TestAuditLoggerWrite(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	audit, err := NewAuditLogger(logFile)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}

	audit.Log("auth_success", "peer-1", "room-1", "10.0.0.1", "password auth", true)
	audit.Log("auth_failure", "peer-2", "room-2", "10.0.0.2", "wrong password", false)

	// Read audit file
	auditFile := logFile + ".audit.json"
	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Verify first entry
	var entry1 AuditLogEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry1); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if entry1.Event != "auth_success" {
		t.Errorf("Event: got %q, want %q", entry1.Event, "auth_success")
	}
	if entry1.PeerID != "peer-1" {
		t.Errorf("PeerID: got %q, want %q", entry1.PeerID, "peer-1")
	}
	if !entry1.Success {
		t.Error("Success should be true")
	}

	// Verify second entry
	var entry2 AuditLogEntry
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if entry2.Event != "auth_failure" {
		t.Errorf("Event: got %q, want %q", entry2.Event, "auth_failure")
	}
	if entry2.Success {
		t.Error("Success should be false")
	}
}

func TestAuditLogEntryOmitEmpty(t *testing.T) {
	entry := AuditLogEntry{
		Event:   "test",
		Success: true,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["peer_id"]; ok {
		t.Error("peer_id should be omitted when empty")
	}
	if _, ok := raw["room_id"]; ok {
		t.Error("room_id should be omitted when empty")
	}
	if _, ok := raw["remote"]; ok {
		t.Error("remote should be omitted when empty")
	}
	if _, ok := raw["detail"]; ok {
		t.Error("detail should be omitted when empty")
	}
}

func TestLoggerWithAuditBuffer(t *testing.T) {
	var buf bytes.Buffer
	audit := &AuditLogger{w: &buf}

	audit.Log("connect", "p1", "r1", "192.168.1.1", "new session", true)

	if buf.Len() == 0 {
		t.Error("expected output in buffer")
	}

	var entry AuditLogEntry
	if err := json.Unmarshal(buf.Bytes()[:buf.Len()-1], &entry); err != nil {
		// Trim newline
		line := strings.TrimSpace(buf.String())
		if err2 := json.Unmarshal([]byte(line), &entry); err2 != nil {
			t.Fatalf("unmarshal: %v (original: %v)", err2, err)
		}
	}

	if entry.Remote != "192.168.1.1" {
		t.Errorf("Remote: got %q, want %q", entry.Remote, "192.168.1.1")
	}
}

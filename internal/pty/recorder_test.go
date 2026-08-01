package pty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRecorder(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir, "test-session-1")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer r.Close()

	if !r.enabled {
		t.Error("recorder should be enabled")
	}

	// Verify file was created
	sessionFile := filepath.Join(dir, "recordings", "test-session-1.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("recording file was not created")
	}
}

func TestRecorderRecordIO(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir, "test-io")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	r.RecordIO("i", []byte("hello"))
	r.RecordIO("o", []byte("world"))
	r.Close()

	// Read back and verify
	sessionFile := filepath.Join(dir, "recordings", "test-io.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "hello") {
		t.Error("recording should contain 'hello'")
	}
	if !strings.Contains(content, "world") {
		t.Error("recording should contain 'world'")
	}
	if !strings.Contains(content, `"e":"i"`) && !strings.Contains(content, `"i"`) {
		t.Error("recording should contain input event marker")
	}
}

func TestRecorderRecordResize(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir, "test-resize")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	r.RecordResize(24, 80)
	r.Close()

	sessionFile := filepath.Join(dir, "recordings", "test-resize.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "24") || !strings.Contains(content, "80") {
		t.Error("recording should contain resize dimensions")
	}
}

func TestRecorderDisabledAfterClose(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir, "test-close")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	r.RecordIO("i", []byte("before"))
	r.Close()
	r.RecordIO("i", []byte("after"))

	sessionFile := filepath.Join(dir, "recordings", "test-close.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "after") {
		t.Error("recording should NOT contain entries after close")
	}
	if !strings.Contains(content, "before") {
		t.Error("recording should contain entries before close")
	}
}

func TestRecorderMultipleSessions(t *testing.T) {
	dir := t.TempDir()
	r1, _ := NewRecorder(dir, "sess-a")
	r2, _ := NewRecorder(dir, "sess-b")

	r1.RecordIO("i", []byte("from-a"))
	r2.RecordIO("i", []byte("from-b"))

	r1.Close()
	r2.Close()

	dataA, _ := os.ReadFile(filepath.Join(dir, "recordings", "sess-a.json"))
	dataB, _ := os.ReadFile(filepath.Join(dir, "recordings", "sess-b.json"))

	if !strings.Contains(string(dataA), "from-a") {
		t.Error("sess-a should contain its data")
	}
	if strings.Contains(string(dataA), "from-b") {
		t.Error("sess-a should NOT contain sess-b data")
	}
	if !strings.Contains(string(dataB), "from-b") {
		t.Error("sess-b should contain its data")
	}
}

func TestRecorderJSONEncoding(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir, "json-test")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	r.RecordIO("i", []byte("test"))
	r.Close()

	data, _ := os.ReadFile(filepath.Join(dir, "recordings", "json-test.json"))

	// Should be valid JSON (array)
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "[") || !strings.HasSuffix(content, "]") {
		t.Error("recording should be a JSON array")
	}
}

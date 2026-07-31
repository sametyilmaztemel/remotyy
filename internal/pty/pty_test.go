package pty

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.sessions) != 0 {
		t.Error("new manager should have no sessions")
	}
}

func TestSpawn(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer sess.Close()

	if sess.PTY == nil {
		t.Error("PTY file should not be nil")
	}
	if sess.pid <= 0 {
		t.Errorf("pid = %d, want > 0", sess.pid)
	}
	if !sess.IsAlive() {
		t.Error("session should be alive after spawn")
	}
}

func TestSpawnWriteRead(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer sess.Close()

	// Write a simple command
	_, err = sess.Write([]byte("echo hello_remotty\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read output
	buf := make([]byte, 4096)
	n, err := sess.ReadWithDeadline(buf, 3*time.Second)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	output := string(buf[:n])
	if len(output) == 0 {
		t.Error("expected some output from shell")
	}
}

func TestResize(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer sess.Close()

	err = sess.Resize(50, 120)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}

	if sess.rows != 50 || sess.cols != 120 {
		t.Errorf("dimensions = %dx%d, want 50x120", sess.rows, sess.cols)
	}
}

func TestClose(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	err = sess.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Session should not be alive after close
	// Give it a moment for the process to exit
	select {
	case <-sess.Done():
		// Good
	case <-time.After(2 * time.Second):
		t.Error("session should be done after close")
	}

	if sess.IsAlive() {
		t.Error("session should not be alive after close")
	}
}

func TestMultipleSessions(t *testing.T) {
	m := NewManager()

	sessions := make([]*Session, 3)
	for i := range sessions {
		var err error
		sessions[i], err = m.Spawn(24, 80)
		if err != nil {
			t.Fatalf("Spawn %d: %v", i, err)
		}
		defer sessions[i].Close()
	}

	m.mu.Lock()
	count := len(m.sessions)
	m.mu.Unlock()

	if count < 3 {
		t.Errorf("expected at least 3 sessions, got %d", count)
	}

	// Close all and verify cleanup
	for _, s := range sessions {
		s.Close()
	}

	// Wait for cleanup goroutines
	time.Sleep(100 * time.Millisecond)
}

func TestSessionDone(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	done := sess.Done()
	if done == nil {
		t.Error("Done() should return a channel")
	}

	sess.Close()

	select {
	case <-done:
		// Good, channel closed
	case <-time.After(2 * time.Second):
		t.Error("Done channel should close after session ends")
	}
}

func TestSessionReadDirect(t *testing.T) {
	m := NewManager()

	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer sess.Close()

	// Write a command
	_, err = sess.Write([]byte("echo read_direct_test\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read using the direct Read method (not ReadWithDeadline)
	buf := make([]byte, 4096)
	done := make(chan struct{})
	var n int
	var readErr error

	go func() {
		n, readErr = sess.Read(buf)
		close(done)
	}()

	select {
	case <-done:
		if readErr != nil {
			t.Fatalf("Read: %v", readErr)
		}
		if n == 0 {
			t.Error("expected some output from Read")
		}
		t.Logf("Read returned %d bytes", n)
	case <-time.After(3 * time.Second):
		t.Fatal("Read timed out")
	}
}

func TestSessionInterface(t *testing.T) {
	// Verify Session implements io.ReadWriter
	m := NewManager()
	sess, err := m.Spawn(24, 80)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer sess.Close()

	// ReadWriter interface check
	var rw interface{} = sess
	if _, ok := rw.(interface {
		Read([]byte) (int, error)
		Write([]byte) (int, error)
	}); !ok {
		t.Error("Session should implement io.ReadWriter")
	}
}

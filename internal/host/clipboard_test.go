package host

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewClipboardMonitor(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)
	if m == nil {
		t.Fatal("expected non-nil ClipboardMonitor")
	}
	if m.running {
		t.Error("should not be running initially")
	}
	if m.stopCh == nil {
		t.Error("stopCh should be initialized")
	}
}

func TestClipboardMonitorOnChange(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	called := false
	m.OnChange(func(text string) {
		called = true
	})

	m.mu.Lock()
	fn := m.onChange
	m.mu.Unlock()

	if fn == nil {
		t.Fatal("onChange callback should be set")
	}
	fn("test")
	if !called {
		t.Error("callback should have been called")
	}
}

func TestClipboardMonitorStartStop(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	// Start should succeed (even if clipboard read fails on headless systems)
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !m.running {
		t.Error("should be running after Start")
	}

	// Give it a moment to start polling
	time.Sleep(100 * time.Millisecond)

	// Stop should halt
	m.Stop()
	if m.running {
		t.Error("should not be running after Stop")
	}
}

func TestClipboardMonitorStartIdempotent(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	if err := m.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Second start should return nil (already running)
	if err := m.Start(); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	m.Stop()
}

func TestClipboardMonitorStopNotRunning(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)
	// Stop on non-running monitor should be safe no-op
	m.Stop()
}

func TestClipboardMonitorGet(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)
	// Get just calls readClipboard which may fail on headless systems
	// We just verify it doesn't panic
	_, _ = m.Get()
}

func TestClipboardMonitorSet(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	err := m.Set("hello")
	// On headless systems without clipboard tools, this will fail
	// but on systems with xclip/etc it should succeed
	if err != nil {
		// Verify the error is about clipboard write
		t.Logf("Set failed (expected on headless): %v", err)
	}
	// If it succeeded, verify lastContent was updated
	if err == nil {
		m.mu.Lock()
		if m.lastContent != "hello" {
			t.Errorf("lastContent = %q, want %q", m.lastContent, "hello")
		}
		m.mu.Unlock()
	}
}

func TestClipboardMonitorCheckClipboard(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	var changedText string
	var mu sync.Mutex
	m.OnChange(func(text string) {
		mu.Lock()
		changedText = text
		mu.Unlock()
	})

	// Set lastContent to something known
	m.lastContent = "old"

	// checkClipboard should read current clipboard and compare
	m.checkClipboard()
	// On systems without clipboard tools, the read will fail silently
	// We just verify no panic
	_ = changedText
}

func TestClipboardMonitorCheckClipboardChangeDetection(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	var changedText string
	var mu sync.Mutex
	m.OnChange(func(text string) {
		mu.Lock()
		changedText = text
		mu.Unlock()
	})

	// If lastContent equals the clipboard content, no change should fire
	m.lastContent = "same"
	m.checkClipboard()

	mu.Lock()
	if changedText != "" {
		t.Error("no change should be detected if content is the same")
	}
	mu.Unlock()
}

func TestClipboardMonitorPollLoopStops(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)
	m.running = true
	m.stopCh = make(chan struct{})

	done := make(chan struct{})
	go func() {
		m.pollLoop()
		close(done)
	}()

	// Stop the poll loop
	close(m.stopCh)

	select {
	case <-done:
		// pollLoop returned
	case <-time.After(2 * time.Second):
		t.Fatal("pollLoop should have stopped")
	}
}

func TestReadClipboard(t *testing.T) {
	// Just ensure readClipboard doesn't panic
	_, err := readClipboard()
	if err != nil {
		t.Logf("readClipboard failed (expected on headless): %v", err)
	}
}

func TestWriteClipboard(t *testing.T) {
	// Just ensure writeClipboard doesn't panic
	err := writeClipboard("test content")
	if err != nil {
		t.Logf("writeClipboard failed (expected on headless): %v", err)
	}
}

func TestClipboardReadCmd(t *testing.T) {
	cmd := clipboardReadCmd()
	// On Linux (which this likely is), it returns a command or nil depending on available tools
	t.Logf("clipboardReadCmd returned: %v", cmd)
}

func TestClipboardWriteCmd(t *testing.T) {
	cmd := clipboardWriteCmd()
	t.Logf("clipboardWriteCmd returned: %v", cmd)
}

func TestLinuxReadCmd(t *testing.T) {
	cmd := linuxReadCmd()
	t.Logf("linuxReadCmd returned: %v", cmd)
}

func TestLinuxWriteCmd(t *testing.T) {
	cmd := linuxWriteCmd()
	t.Logf("linuxWriteCmd returned: %v", cmd)
}

func TestToolExists(t *testing.T) {
	// "ls" should exist on any Unix system
	if !toolExists("ls") {
		t.Error("ls should exist")
	}
	// A made-up tool should not exist
	if toolExists("definitely_not_a_real_tool_xyz123") {
		t.Error("nonexistent tool should not exist")
	}
}

func TestClipboardMonitorStartAndPoll(t *testing.T) {
	log := zerolog.Nop()
	m := NewClipboardMonitor(log)

	var changeMu sync.Mutex
	var changes []string
	m.OnChange(func(text string) {
		changeMu.Lock()
		changes = append(changes, text)
		changeMu.Unlock()
	})

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Let it poll at least once
	time.Sleep(600 * time.Millisecond)

	// No assertions on changes since clipboard content is environment-dependent
	t.Logf("clipboard changes detected: %v", changes)
}

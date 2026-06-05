// Package host implements the remotty host daemon.
package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/protocol"
	"github.com/sametyilmaztemel/remotty/internal/webrtc"
)

// clipboardPollInterval is how often we check for clipboard changes.
const clipboardPollInterval = 500 * time.Millisecond

// ClipboardMonitor watches the system clipboard for changes and can
// write clipboard data to the system clipboard.
type ClipboardMonitor struct {
	mu          sync.Mutex
	lastContent string
	log         zerolog.Logger
	stopCh      chan struct{}
	running     bool
	onChange    func(text string) // called when clipboard content changes
}

// NewClipboardMonitor creates a new clipboard monitor.
func NewClipboardMonitor(log zerolog.Logger) *ClipboardMonitor {
	return &ClipboardMonitor{
		log:    log.With().Str("component", "clipboard").Logger(),
		stopCh: make(chan struct{}),
	}
}

// OnChange registers a callback for clipboard changes.
func (m *ClipboardMonitor) OnChange(fn func(text string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// Start begins polling the clipboard for changes.
func (m *ClipboardMonitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	m.running = true
	m.stopCh = make(chan struct{})

	// Read initial clipboard content
	content, err := readClipboard()
	if err != nil {
		m.log.Warn().Err(err).Msg("Failed to read initial clipboard content")
	}
	m.lastContent = content

	go m.pollLoop()
	m.log.Info().Msg("Clipboard monitor started")
	return nil
}

// Stop halts clipboard polling.
func (m *ClipboardMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	m.log.Info().Msg("Clipboard monitor stopped")
}

// Get returns the current clipboard content.
func (m *ClipboardMonitor) Get() (string, error) {
	return readClipboard()
}

// Set writes text to the system clipboard and updates the last known content.
func (m *ClipboardMonitor) Set(text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := writeClipboard(text); err != nil {
		return err
	}
	m.lastContent = text
	return nil
}

// pollLoop periodically checks clipboard for changes.
func (m *ClipboardMonitor) pollLoop() {
	ticker := time.NewTicker(clipboardPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkClipboard()
		}
	}
}

// checkClipboard reads the clipboard and fires onChange if it changed.
func (m *ClipboardMonitor) checkClipboard() {
	content, err := readClipboard()
	if err != nil {
		return
	}

	m.mu.Lock()
	hasChanged := content != m.lastContent && content != ""
	if hasChanged {
		m.lastContent = content
	}
	onChange := m.onChange
	m.mu.Unlock()

	if hasChanged && onChange != nil {
		m.log.Debug().Int("len", len(content)).Msg("Clipboard content changed")
		onChange(content)
	}
}

// readClipboard reads the system clipboard using platform-specific tools.
func readClipboard() (string, error) {
	cmd := clipboardReadCmd()
	if cmd == nil {
		return "", fmt.Errorf("clipboard read not supported on this platform")
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("read clipboard: %w", err)
	}
	return strings.TrimRight(out.String(), "\n\r"), nil
}

// writeClipboard writes text to the system clipboard.
func writeClipboard(text string) error {
	cmd := clipboardWriteCmd()
	if cmd == nil {
		return fmt.Errorf("clipboard write not supported on this platform")
	}
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}
	return nil
}

// clipboardReadCmd returns the platform-specific command to read clipboard.
// Supports: macOS (pbpaste), Linux (wl-paste, xclip, xsel).
func clipboardReadCmd() *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pbpaste")
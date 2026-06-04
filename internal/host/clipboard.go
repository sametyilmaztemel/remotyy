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
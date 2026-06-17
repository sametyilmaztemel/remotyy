package pty

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RecordEntry is a single recorded terminal event.
type RecordEntry struct {
	Timestamp int64  `json:"t"` // milliseconds since session start
	Event     string `json:"e"` // "i" = input, "o" = output, "r" = resize
	Data      string `json:"d,omitempty"`
	Rows      uint16 `json:"r,omitempty"`
	Cols      uint16 `json:"c,omitempty"`
}

// Recorder records terminal sessions for playback.
type Recorder struct {
	mu       sync.Mutex
	start    time.Time
	entries  []RecordEntry
	file     *os.File
	enabled  bool
}

// NewRecorder creates a recorder.
func NewRecorder(dataDir, sessionID string) (*Recorder, error) {
	dir := filepath.Join(dataDir, "recordings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create recordings dir: %w", err)
	}

	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s.json", sessionID)))
	if err != nil {
		return nil, err
	}

	return &Recorder{
		start:   time.Now(),
		file:    f,
		enabled: true,
	}, nil
}

// RecordIO records an input or output event.
func (r *Recorder) RecordIO(event string, data []byte) {
	if !r.enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, RecordEntry{
		Timestamp: time.Since(r.start).Milliseconds(),
		Event:     event,
		Data:      string(data),
	})
}

// RecordResize records a resize event.
func (r *Recorder) RecordResize(rows, cols uint16) {
	if !r.enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, RecordEntry{
		Timestamp: time.Since(r.start).Milliseconds(),
		Event:     "r",
		Rows:      rows,
		Cols:      cols,
	})
}

// Close flushes and closes the recording.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = false
	enc := json.NewEncoder(r.file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r.entries); err != nil {
		return err
	}
	return r.file.Close()
}

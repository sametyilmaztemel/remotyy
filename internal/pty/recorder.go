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
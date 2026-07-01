// Package transfer provides file transfer capabilities over WebRTC data channels.
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sametyilmaztemel/remotty/internal/protocol"
)

const DefaultChunkSize = 65536 // 64KB

// TransferState tracks a file transfer.
type TransferState int

const (
	TransferPending  TransferState = iota
	TransferActive
	TransferPaused
	TransferComplete
	TransferFailed
	TransferCancelled
)

// Transfer represents an active or completed file transfer.
type Transfer struct {
	ID           string
	Name         string
	Path         string
	Size         int64
	MimeType     string
	Direction    string // "send" or "receive"
	State        TransferState
	ChunkSize    int
	TotalChunks  int
	ReceivedChunks int
	BytesSent    int64
	StartedAt    time.Time
	CompletedAt  time.Time
	mu           sync.Mutex
}

// Manager handles file transfers.
type Manager struct {
	transfers map[string]*Transfer
	dataDir   string
	mu        sync.Mutex
	onProgress func(*Transfer)
}

// NewManager creates a transfer manager.
func NewManager(dataDir string) *Manager {
	return &Manager{
		transfers: make(map[string]*Transfer),
		dataDir:   dataDir,
	}
}

// OnProgress sets the progress callback.
func (m *Manager) OnProgress(fn func(*Transfer)) {
	m.onProgress = fn
}

// InitiateSend starts sending a file.
func (m *Manager) InitiateSend(path string) (*Transfer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	t := &Transfer{
		ID:        fmt.Sprintf("tf-%d", time.Now().UnixNano()),
		Name:      filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		Direction: "send",
		State:     TransferPending,
		ChunkSize: DefaultChunkSize,
		TotalChunks: int((info.Size() + int64(DefaultChunkSize) - 1) / int64(DefaultChunkSize)),
		StartedAt: time.Now(),
	}

	m.mu.Lock()
	m.transfers[t.ID] = t
	m.mu.Unlock()

	return t, nil
}

// InitiateReceive prepares to receive a file.
func (m *Manager) InitiateReceive(req protocol.FileRequestPayload) (*Transfer, error) {
	dir := filepath.Join(m.dataDir, "downloads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	t := &Transfer{
		ID:          req.TransferID,
		Name:        req.Name,
		Path:        filepath.Join(dir, req.Name),
		Size:        req.Size,
		MimeType:    req.MimeType,
		Direction:   "receive",
		State:       TransferActive,
		ChunkSize:   req.ChunkSize,
		TotalChunks: int((req.Size + int64(req.ChunkSize) - 1) / int64(req.ChunkSize)),
		StartedAt:   time.Now(),
	}

	m.mu.Lock()
	m.transfers[t.ID] = t
	m.mu.Unlock()

	return t, nil
}

// WriteChunk writes a received chunk to disk.
func (t *Transfer) WriteChunk(index int, data []byte, checksum string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State != TransferActive {
		return fmt.Errorf("transfer not active")
	}

	// Verify checksum
	if checksum != "" {
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != checksum {
			return fmt.Errorf("checksum mismatch on chunk %d", index)
		}
	}

	// Append to file
	f, err := os.OpenFile(t.Path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	offset := int64(index) * int64(t.ChunkSize)
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}

	t.ReceivedChunks++
	t.BytesSent += int64(len(data))

	return nil
}

// ReadChunk reads a chunk for sending.
func (t *Transfer) ReadChunk(index int) ([]byte, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.State == TransferCancelled {
		return nil, "", fmt.Errorf("transfer cancelled")
	}

	f, err := os.Open(t.Path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	offset := int64(index) * int64(t.ChunkSize)
	f.Seek(offset, io.SeekStart)

	data := make([]byte, t.ChunkSize)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return nil, "", err
	}
	data = data[:n]

	h := sha256.Sum256(data)
	checksum := hex.EncodeToString(h[:])

	return data, checksum, nil
}

// Complete marks a transfer as complete.
func (t *Transfer) Complete() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.State = TransferComplete
	t.CompletedAt = time.Now()
}

// Cancel cancels a transfer.
func (t *Transfer) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.State = TransferCancelled
}

// Progress returns transfer progress (0.0 to 1.0).
func (t *Transfer) Progress() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.TotalChunks == 0 {
		return 0
	}
	return float64(t.ReceivedChunks) / float64(t.TotalChunks)
}
// Get returns a transfer by ID.
func (m *Manager) Get(id string) *Transfer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.transfers[id]
}

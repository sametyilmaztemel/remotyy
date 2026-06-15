// Package pty manages pseudoterminal sessions for remote terminal access.
package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/rs/zerolog/log"
)

// Session represents an active PTY session with a shell process.
type Session struct {
	PTY    *os.File
	cmd    *exec.Cmd
	pid    int
	rows   uint16
	cols   uint16
	created time.Time
	mu     sync.Mutex
	done   chan struct{}
}

// Manager creates and manages PTY sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.Mutex
	nextID   int
}

// NewManager creates a PTY session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Spawn starts a new shell session with the given terminal size.
func (m *Manager) Spawn(rows, cols uint16) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
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
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		fmt.Sprintf("LINES=%d", rows),
		fmt.Sprintf("COLUMNS=%d", cols),
	)

	f, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	s := &Session{
		PTY:     f,
		cmd:     cmd,
		pid:     cmd.Process.Pid,
		rows:    rows,
		cols:    cols,
		created: time.Now(),
		done:    make(chan struct{}),
	}

	sessionID := fmt.Sprintf("sess-%d", m.nextID)
	m.nextID++

	m.mu.Lock()
	m.sessions[sessionID] = s
	m.mu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Debug().Err(err).Int("pid", s.pid).Msg("Shell process exited")
		}
		close(s.done)
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()
	}()

	log.Debug().Int("pid", s.pid).Str("shell", shell).Msg("PTY session started")
	return s, nil
}

// Read reads output from the PTY (stdout of shell process).
func (s *Session) Read(buf []byte) (int, error) {
	return s.PTY.Read(buf)
}

// ReadWithDeadline reads with a timeout.
func (s *Session) ReadWithDeadline(buf []byte, timeout time.Duration) (int, error) {
	s.PTY.SetReadDeadline(time.Now().Add(timeout))
	n, err := s.PTY.Read(buf)
	s.PTY.SetReadDeadline(time.Time{})
	return n, err
}

// Write writes input to the PTY (stdin of shell process).
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.PTY.Write(data)
}

// Resize changes the terminal window dimensions.
func (s *Session) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = rows
	s.cols = cols
	return pty.Setsize(s.PTY, &pty.Winsize{Rows: rows, Cols: cols})
}

// Close terminates the session.
func (s *Session) Close() error {
	s.cmd.Process.Kill()
	return s.PTY.Close()
}

// Done returns a channel that closes when the session ends.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// IsAlive checks if the session is still running.
func (s *Session) IsAlive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Ensure Session implements io.ReadWriter.
var _ io.ReadWriter = (*Session)(nil)

package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

// Session represents an active terminal session.
type Session struct {
	ID        string
	UserID    string
	OrgID     string
	ProjectID string
	JWT       string
	PTY       *os.File
	Cmd       *exec.Cmd
	Created   time.Time
	LastUsed  time.Time
	Cols      uint16
	Rows      uint16

	mu     sync.Mutex
	closed bool
}

// Write sends data to the terminal's stdin.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	s.LastUsed = time.Now()
	return s.PTY.Write(data)
}

// Read reads data from the terminal's stdout.
func (s *Session) Read(buf []byte) (int, error) {
	return s.PTY.Read(buf)
}

// Resize changes the terminal dimensions.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	s.Cols = cols
	s.Rows = rows
	return pty.Setsize(s.PTY, &pty.Winsize{Cols: cols, Rows: rows})
}

// Close terminates the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	// Kill the process
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}

	// Close PTY
	if s.PTY != nil {
		s.PTY.Close()
	}

	return nil
}

// Manager handles terminal session lifecycle.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	cliBin   string
	server   string
}

// NewManager creates a new terminal manager.
func NewManager(cliBin, server string) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		cliBin:   cliBin,
		server:   server,
	}
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "term_" + hex.EncodeToString(b)
}

// CreateSession spawns a new terminal session.
func (m *Manager) CreateSession(userID, orgID, projectID, jwt string, cols, rows uint16) (*Session, error) {
	sessionID := generateSessionID()

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		OrgID:     orgID,
		ProjectID: projectID,
		JWT:       jwt,
		Created:   time.Now(),
		LastUsed:  time.Now(),
		Cols:      cols,
		Rows:      rows,
	}

	// Build CLI command with minimal environment (do not inherit server env)
	cmd := exec.Command(m.cliBin, "shell")
	cmd.Env = []string{
		"NOTIF_JWT=" + jwt,
		"NOTIF_SERVER=" + m.server,
		"NOTIF_PROJECT_ID=" + projectID,
		"TERM=xterm-256color",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
	}

	// Start with PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	session.PTY = ptmx
	session.Cmd = cmd

	// Register session
	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	slog.Info("terminal session created",
		"session_id", sessionID,
		"user_id", userID,
		"cols", cols,
		"rows", rows,
	)

	// Wait for process exit in background
	go func() {
		cmd.Wait()
		m.CloseSession(sessionID)
	}()

	return session, nil
}

// GetSession retrieves an existing session.
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[sessionID]
	return session, ok
}

// CloseSession terminates and removes a session.
func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}

	slog.Info("terminal session closed", "session_id", sessionID)
	return session.Close()
}

// CleanupStaleSessions removes sessions that haven't been used recently.
func (m *Manager) CleanupStaleSessions(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, session := range m.sessions {
		if now.Sub(session.LastUsed) > maxAge {
			slog.Info("cleaning up stale terminal session", "session_id", id)
			session.Close()
			delete(m.sessions, id)
		}
	}
}

// SessionCount returns the number of active sessions.
func (m *Manager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// UserSessionCount returns the number of sessions for a specific user.
func (m *Manager) UserSessionCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, session := range m.sessions {
		if session.UserID == userID {
			count++
		}
	}
	return count
}

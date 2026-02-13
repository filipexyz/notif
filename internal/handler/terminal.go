package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/terminal"
	ws "github.com/gorilla/websocket"
)

const (
	// Max sessions per user
	maxSessionsPerUser = 3
	// Read/write buffer size
	terminalBufferSize = 4096
	// Write wait time
	writeWait = 10 * time.Second
	// Pong wait time
	pongWait = 60 * time.Second
	// Ping period
	pingPeriod = (pongWait * 9) / 10
)

// Terminal message types
type terminalMessage struct {
	Type      string `json:"type"`
	Data      string `json:"data,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Cols      uint16 `json:"cols,omitempty"`
	Rows      uint16 `json:"rows,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// TerminalHandler handles terminal WebSocket connections.
type TerminalHandler struct {
	manager  *terminal.Manager
	upgrader ws.Upgrader
}

// NewTerminalHandler creates a new terminal handler.
func NewTerminalHandler(manager *terminal.Manager, allowedOrigins []string) *TerminalHandler {
	return &TerminalHandler{
		manager:  manager,
		upgrader: newUpgrader(allowedOrigins),
	}
}

// HandleWS handles WebSocket connections for terminal sessions.
func (h *TerminalHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	// Get auth context (accepts both Clerk JWT and API key for self-hosted)
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Must have either UserID (JWT) or APIKeyID (self-hosted)
	if authCtx.UserID == nil && authCtx.APIKeyID == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get token from query param (JWT for cloud, API key for self-hosted)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	// Get user/key identifier for session tracking
	var sessionOwner string
	if authCtx.UserID != nil {
		sessionOwner = *authCtx.UserID
	} else if authCtx.APIKeyID != nil {
		sessionOwner = authCtx.APIKeyID.String()
	}

	// Check max sessions per user/key
	if h.manager.UserSessionCount(sessionOwner) >= maxSessionsPerUser {
		http.Error(w, "max sessions reached", http.StatusTooManyRequests)
		return
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("terminal websocket upgrade failed", "error", err)
		return
	}

	slog.Info("terminal websocket connected", "session_owner", sessionOwner, "project_id", authCtx.ProjectID)

	// Handle connection
	h.handleConnection(conn, sessionOwner, authCtx.OrgID, authCtx.ProjectID, token)
}

func (h *TerminalHandler) handleConnection(conn *ws.Conn, userID, orgID, projectID, jwt string) {
	defer conn.Close()

	// Wait for connect message with terminal size
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var connectMsg terminalMessage
	if err := conn.ReadJSON(&connectMsg); err != nil {
		slog.Error("failed to read connect message", "error", err)
		return
	}

	if connectMsg.Type != "connect" {
		h.sendError(conn, "INVALID_MESSAGE", "expected connect message")
		return
	}

	// Default terminal size
	cols := connectMsg.Cols
	rows := connectMsg.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	// Create session
	session, err := h.manager.CreateSession(userID, orgID, projectID, jwt, cols, rows)
	if err != nil {
		slog.Error("failed to create terminal session", "error", err)
		h.sendError(conn, "SESSION_ERROR", err.Error())
		return
	}

	// Send connected confirmation
	conn.WriteJSON(terminalMessage{
		Type:      "connected",
		SessionID: session.ID,
	})

	// Reset read deadline
	conn.SetReadDeadline(time.Time{})

	// Bridge WebSocket <-> PTY
	h.bridge(conn, session)
}

func (h *TerminalHandler) bridge(conn *ws.Conn, session *terminal.Session) {
	done := make(chan struct{})

	// Read from PTY -> Send to WebSocket
	go func() {
		defer close(done)
		buf := make([]byte, terminalBufferSize)
		for {
			n, err := session.Read(buf)
			if err != nil {
				conn.WriteJSON(terminalMessage{
					Type:   "closed",
					Reason: "session ended",
				})
				return
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteJSON(terminalMessage{
				Type: "output",
				Data: string(buf[:n]),
			}); err != nil {
				return
			}
		}
	}()

	// Setup ping/pong
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Ping ticker
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(ws.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read from WebSocket -> Write to PTY
	for {
		var msg terminalMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "input":
			session.Write([]byte(msg.Data))
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				session.Resize(msg.Cols, msg.Rows)
			}
		case "disconnect":
			h.manager.CloseSession(session.ID)
			return
		}
	}

	// Cleanup
	h.manager.CloseSession(session.ID)
}

func (h *TerminalHandler) sendError(conn *ws.Conn, code, message string) {
	conn.WriteJSON(terminalMessage{
		Type:    "error",
		Code:    code,
		Message: message,
	})
}

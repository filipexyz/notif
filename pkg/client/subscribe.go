package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum reconnection attempts before giving up.
	maxReconnectAttempts = 0 // 0 = infinite

	// Initial reconnection delay.
	initialReconnectDelay = 1 * time.Second

	// Maximum reconnection delay.
	maxReconnectDelay = 30 * time.Second
)

// SubscribeOptions configures the subscription.
type SubscribeOptions struct {
	AutoAck bool
	Group   string
	From    string // "latest", "beginning", or timestamp
}

// Event represents a received event.
type Event struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
	Attempt   int             `json:"attempt,omitempty"`
}

// Subscription represents an active subscription with auto-reconnection.
type Subscription struct {
	client  *Client
	topics  []string
	opts    SubscribeOptions
	conn    *websocket.Conn
	connMu  sync.RWMutex
	events  chan *Event
	errors  chan error
	done    chan struct{}
	closed  bool
	closeMu sync.Mutex
}

// Subscribe connects to the WebSocket and subscribes to topics.
// The subscription will automatically reconnect on connection loss.
func (c *Client) Subscribe(ctx context.Context, topics []string, opts SubscribeOptions) (*Subscription, error) {
	sub := &Subscription{
		client: c,
		topics: topics,
		opts:   opts,
		events: make(chan *Event, 100),
		errors: make(chan error, 10),
		done:   make(chan struct{}),
	}

	// Initial connection
	if err := sub.connect(ctx); err != nil {
		return nil, err
	}

	// Start read and write pumps
	go sub.readPump()
	go sub.writePump()

	return sub, nil
}

func (s *Subscription) connect(ctx context.Context) error {
	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(s.client.server, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/ws"
	if s.client.projectID != "" {
		wsURL += "?project_id=" + s.client.projectID
	}

	// Set up headers with auth
	header := http.Header{}
	header.Set("Authorization", "Bearer "+s.client.apiKey)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return &ConnectionError{Err: err}
	}

	// Configure connection
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	s.connMu.Lock()
	s.conn = conn
	s.connMu.Unlock()

	// Send subscribe message
	subscribeMsg := map[string]any{
		"action": "subscribe",
		"topics": s.topics,
		"options": map[string]any{
			"auto_ack": s.opts.AutoAck,
			"group":    s.opts.Group,
			"from":     s.opts.From,
		},
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		conn.Close()
		return err
	}

	return nil
}

func (s *Subscription) reconnect() {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return
	}
	s.closeMu.Unlock()

	delay := initialReconnectDelay
	attempts := 0

	for {
		select {
		case <-s.done:
			return
		default:
		}

		// Wait before reconnecting
		select {
		case <-s.done:
			return
		case <-time.After(delay):
		}

		attempts++
		if maxReconnectAttempts > 0 && attempts > maxReconnectAttempts {
			select {
			case s.errors <- &ConnectionError{Err: ErrMaxReconnectAttempts}:
			default:
			}
			return
		}

		// Try to reconnect
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.connect(ctx)
		cancel()

		if err == nil {
			// Successfully reconnected, restart pumps
			// Send reconnection success to errors channel so CLI can log it
			select {
			case s.errors <- &ReconnectedError{}:
			default:
			}
			go s.readPump()
			go s.writePump()
			return
		}

		// Report error and increase delay
		select {
		case s.errors <- err:
		default:
		}

		delay *= 2
		if delay > maxReconnectDelay {
			delay = maxReconnectDelay
		}
	}
}

func (s *Subscription) readPump() {
	defer func() {
		s.connMu.RLock()
		conn := s.conn
		s.connMu.RUnlock()
		if conn != nil {
			conn.Close()
		}
	}()

	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.connMu.RLock()
		conn := s.conn
		s.connMu.RUnlock()

		if conn == nil {
			return
		}

		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}

			// Check if we should reconnect
			s.closeMu.Lock()
			closed := s.closed
			s.closeMu.Unlock()

			if !closed {
				// Report error
				select {
				case s.errors <- err:
				default:
				}
				// Trigger reconnection
				go s.reconnect()
			}
			return
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "event":
			event := &Event{
				ID:    msg["id"].(string),
				Topic: msg["topic"].(string),
			}
			if data, ok := msg["data"]; ok {
				event.Data, _ = json.Marshal(data)
			}
			if ts, ok := msg["timestamp"].(string); ok {
				event.Timestamp, _ = time.Parse(time.RFC3339, ts)
			}
			if attempt, ok := msg["attempt"].(float64); ok {
				event.Attempt = int(attempt)
			}

			select {
			case s.events <- event:
			case <-s.done:
				return
			}

		case "subscribed":
			// Subscription confirmed, continue

		case "error":
			errMsg := "unknown error"
			if m, ok := msg["message"].(string); ok {
				errMsg = m
			}
			select {
			case s.errors <- &APIError{Message: errMsg}:
			default:
			}
		}
	}
}

func (s *Subscription) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.connMu.RLock()
			conn := s.conn
			s.connMu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// Connection lost, readPump will handle reconnection
				return
			}
		}
	}
}

// Events returns the channel of received events.
func (s *Subscription) Events() <-chan *Event {
	return s.events
}

// Errors returns the channel of errors.
// Errors are non-fatal; the subscription will attempt to reconnect.
func (s *Subscription) Errors() <-chan error {
	return s.errors
}

// Ack acknowledges an event.
func (s *Subscription) Ack(eventID string) error {
	s.connMu.RLock()
	conn := s.conn
	s.connMu.RUnlock()

	if conn == nil {
		return &ConnectionError{Err: ErrNotConnected}
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(map[string]string{
		"action": "ack",
		"id":     eventID,
	})
}

// Nack negative-acknowledges an event.
func (s *Subscription) Nack(eventID string, retryIn string) error {
	s.connMu.RLock()
	conn := s.conn
	s.connMu.RUnlock()

	if conn == nil {
		return &ConnectionError{Err: ErrNotConnected}
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(map[string]any{
		"action":   "nack",
		"id":       eventID,
		"retry_in": retryIn,
	})
}

// Close closes the subscription.
func (s *Subscription) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	s.closeMu.Unlock()

	close(s.done)

	s.connMu.RLock()
	conn := s.conn
	s.connMu.RUnlock()

	if conn != nil {
		return conn.Close()
	}
	return nil
}

// IsConnected returns true if the subscription is currently connected.
func (s *Subscription) IsConnected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conn != nil
}

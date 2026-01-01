package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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

// Subscription represents an active subscription.
type Subscription struct {
	conn     *websocket.Conn
	events   chan *Event
	errors   chan error
	done     chan struct{}
	autoAck  bool
}

// Subscribe connects to the WebSocket and subscribes to topics.
func (c *Client) Subscribe(ctx context.Context, topics []string, opts SubscribeOptions) (*Subscription, error) {
	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(c.server, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/subscribe"

	// Set up headers with auth
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.apiKey)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}

	sub := &Subscription{
		conn:    conn,
		events:  make(chan *Event, 100),
		errors:  make(chan error, 1),
		done:    make(chan struct{}),
		autoAck: opts.AutoAck,
	}

	// Send subscribe message
	subscribeMsg := map[string]any{
		"action": "subscribe",
		"topics": topics,
		"options": map[string]any{
			"auto_ack": opts.AutoAck,
			"group":    opts.Group,
			"from":     opts.From,
		},
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		conn.Close()
		return nil, err
	}

	// Start reading messages
	go sub.readPump()

	return sub, nil
}

func (s *Subscription) readPump() {
	defer close(s.events)
	defer s.conn.Close()

	for {
		select {
		case <-s.done:
			return
		default:
		}

		var msg map[string]any
		if err := s.conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			select {
			case s.errors <- err:
			default:
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

// Events returns the channel of received events.
func (s *Subscription) Events() <-chan *Event {
	return s.events
}

// Errors returns the channel of errors.
func (s *Subscription) Errors() <-chan error {
	return s.errors
}

// Ack acknowledges an event.
func (s *Subscription) Ack(eventID string) error {
	return s.conn.WriteJSON(map[string]string{
		"action": "ack",
		"id":     eventID,
	})
}

// Nack negative-acknowledges an event.
func (s *Subscription) Nack(eventID string, retryIn string) error {
	return s.conn.WriteJSON(map[string]any{
		"action":   "nack",
		"id":       eventID,
		"retry_in": retryIn,
	})
}

// Close closes the subscription.
func (s *Subscription) Close() error {
	close(s.done)
	return s.conn.Close()
}

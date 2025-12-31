package websocket

import (
	"encoding/json"
	"time"
)

// Client to Server messages

type ClientMessage struct {
	Action string `json:"action"`
}

type SubscribeMessage struct {
	Action  string            `json:"action"`
	Topics  []string          `json:"topics"`
	Options SubscribeOptions  `json:"options,omitempty"`
}

type SubscribeOptions struct {
	AutoAck    bool   `json:"auto_ack"`
	From       string `json:"from,omitempty"` // "latest", "beginning", or timestamp
	Group      string `json:"group,omitempty"`
	MaxRetries int    `json:"max_retries,omitempty"`
	AckTimeout string `json:"ack_timeout,omitempty"`
}

type AckMessage struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

type NackMessage struct {
	Action  string `json:"action"`
	ID      string `json:"id"`
	RetryIn string `json:"retry_in,omitempty"`
}

// Server to Client messages

type ServerMessage struct {
	Type string `json:"type"`
}

type EventMessage struct {
	Type        string          `json:"type"`
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	Data        json.RawMessage `json:"data"`
	Timestamp   time.Time       `json:"timestamp"`
	Attempt     int             `json:"attempt,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

type SubscribedMessage struct {
	Type       string   `json:"type"`
	Topics     []string `json:"topics"`
	ConsumerID string   `json:"consumer_id,omitempty"`
}

type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PongMessage struct {
	Type string `json:"type"`
}

// NewEventMessage creates an event message from domain event.
func NewEventMessage(id, topic string, data json.RawMessage, timestamp time.Time, attempt, maxAttempts int) *EventMessage {
	return &EventMessage{
		Type:        "event",
		ID:          id,
		Topic:       topic,
		Data:        data,
		Timestamp:   timestamp,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
	}
}

// NewSubscribedMessage creates a subscribed confirmation.
func NewSubscribedMessage(topics []string, consumerID string) *SubscribedMessage {
	return &SubscribedMessage{
		Type:       "subscribed",
		Topics:     topics,
		ConsumerID: consumerID,
	}
}

// NewErrorMessage creates an error message.
func NewErrorMessage(code, message string) *ErrorMessage {
	return &ErrorMessage{
		Type:    "error",
		Code:    code,
		Message: message,
	}
}

// NewPongMessage creates a pong response.
func NewPongMessage() *PongMessage {
	return &PongMessage{Type: "pong"}
}

// ParseDuration parses duration strings like "5m", "30s", "1h".
func ParseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute // default
	}
	return d
}

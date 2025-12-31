package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
	Attempt   int             `json:"attempt,omitempty"`
}

// NewEvent creates a new event with a generated ID.
func NewEvent(topic string, data json.RawMessage) *Event {
	return &Event{
		ID:        generateEventID(),
		Topic:     topic,
		Data:      data,
		Timestamp: time.Now().UTC(),
		Attempt:   1,
	}
}

// generateEventID creates a unique event ID with "evt_" prefix.
func generateEventID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "evt_" + hex.EncodeToString(b)
}

// EmitRequest is the request body for POST /emit.
type EmitRequest struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

// EmitResponse is the response body for POST /emit.
type EmitResponse struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
}

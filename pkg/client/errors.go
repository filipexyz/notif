package client

import (
	"errors"
	"fmt"
)

// Sentinel errors for connection handling.
var (
	ErrNotConnected         = errors.New("not connected")
	ErrMaxReconnectAttempts = errors.New("max reconnect attempts reached")
)

// APIError represents an error from the API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error: %s", e.Message)
}

// AuthError represents an authentication error.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error: %s", e.Message)
}

// ConnectionError represents a connection failure.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

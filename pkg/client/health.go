package client

import (
	"encoding/json"
	"net/http"
)

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// Health checks the server health.
func (c *Client) Health() (*HealthResponse, error) {
	resp, err := c.httpClient.Get(c.server + "/health")
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "health check failed",
		}
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}

	return &health, nil
}

// Ready checks if the server is ready.
func (c *Client) Ready() error {
	resp, err := c.httpClient.Get(c.server + "/ready")
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "server not ready",
		}
	}

	return nil
}

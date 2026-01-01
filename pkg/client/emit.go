package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// EmitRequest represents the request to emit an event.
type EmitRequest struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

// EmitResponse represents the response from emit.
type EmitResponse struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
}

// Emit publishes an event to a topic.
func (c *Client) Emit(topic string, data json.RawMessage) (*EmitResponse, error) {
	req := EmitRequest{
		Topic: topic,
		Data:  data,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.server+"/api/v1/emit", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Message: "invalid or missing API key"}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Error
		if msg == "" {
			msg = "emit failed"
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	var emitResp EmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&emitResp); err != nil {
		return nil, err
	}

	return &emitResp, nil
}

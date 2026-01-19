package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// StoredEvent represents an event with its stream metadata.
type StoredEvent struct {
	Seq   uint64 `json:"seq"`
	Event struct {
		ID        string          `json:"id"`
		Topic     string          `json:"topic"`
		Data      json.RawMessage `json:"data"`
		Timestamp time.Time       `json:"timestamp"`
	} `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// EventsListResponse is the response from listing events.
type EventsListResponse struct {
	Events []StoredEvent `json:"events"`
	Count  int           `json:"count"`
}

// EventsQueryOptions configures event queries.
type EventsQueryOptions struct {
	Topic string
	From  time.Time
	To    time.Time
	Limit int
}

// EventsList queries historical events.
func (c *Client) EventsList(opts EventsQueryOptions) (*EventsListResponse, error) {
	u, _ := url.Parse(c.server + "/api/v1/events")
	q := u.Query()

	if opts.Topic != "" {
		q.Set("topic", opts.Topic)
	}
	if !opts.From.IsZero() {
		q.Set("from", opts.From.Format(time.RFC3339))
	}
	if !opts.To.IsZero() {
		q.Set("to", opts.To.Format(time.RFC3339))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Message: "invalid or missing API key"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to list events"}
	}

	var result EventsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// EventsGet retrieves a specific event by sequence number.
func (c *Client) EventsGet(seq uint64) (*StoredEvent, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/events/%d", c.server, seq), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "event not found"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get event"}
	}

	var event StoredEvent
	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		return nil, err
	}

	return &event, nil
}

// EventsStatsResponse is the response from events stats.
type EventsStatsResponse struct {
	Messages   uint64    `json:"messages"`
	Bytes      uint64    `json:"bytes"`
	FirstSeq   uint64    `json:"first_seq"`
	LastSeq    uint64    `json:"last_seq"`
	FirstTime  time.Time `json:"first_time"`
	LastTime   time.Time `json:"last_time"`
	Consumers  int       `json:"consumers"`
}

// EventsStats returns stream statistics.
func (c *Client) EventsStats() (*EventsStatsResponse, error) {
	req, err := http.NewRequest("GET", c.server+"/api/v1/events/stats", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get stats"}
	}

	var stats EventsStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

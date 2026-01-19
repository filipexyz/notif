package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DLQMessage represents a message in the dead letter queue.
type DLQMessage struct {
	ID            string          `json:"id"`
	OriginalTopic string          `json:"original_topic"`
	Data          json.RawMessage `json:"data"`
	Timestamp     time.Time       `json:"timestamp"`
	FailedAt      time.Time       `json:"failed_at"`
	Attempts      int             `json:"attempts"`
	LastError     string          `json:"last_error,omitempty"`
	ConsumerGroup string          `json:"consumer_group,omitempty"`
}

// DLQEntry represents a DLQ message with its sequence number.
type DLQEntry struct {
	Seq     uint64      `json:"seq"`
	Subject string      `json:"subject"`
	Message *DLQMessage `json:"message"`
}

// DLQListResponse is the response from listing DLQ messages.
type DLQListResponse struct {
	Messages []DLQEntry `json:"messages"`
	Count    int        `json:"count"`
}

// DLQList lists messages in the dead letter queue.
func (c *Client) DLQList(topic string, limit int) (*DLQListResponse, error) {
	u, _ := url.Parse(c.server + "/api/v1/dlq")
	q := u.Query()
	if topic != "" {
		q.Set("topic", topic)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
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
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to list DLQ"}
	}

	var result DLQListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DLQGet retrieves a specific DLQ message.
func (c *Client) DLQGet(seq uint64) (*DLQEntry, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/dlq/%d", c.server, seq), nil)
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
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "message not found"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get DLQ message"}
	}

	var entry DLQEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// DLQReplay replays a DLQ message to its original topic.
func (c *Client) DLQReplay(seq uint64) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/dlq/%d/replay", c.server, seq), nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Message: "failed to replay message"}
	}

	return nil
}

// DLQDelete removes a message from the DLQ.
func (c *Client) DLQDelete(seq uint64) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/dlq/%d", c.server, seq), nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Message: "failed to delete message"}
	}

	return nil
}

// DLQReplayAllResponse is the response from replay-all.
type DLQReplayAllResponse struct {
	Replayed int `json:"replayed"`
	Failed   int `json:"failed"`
}

// DLQReplayAll replays all messages from the DLQ.
func (c *Client) DLQReplayAll(topic string) (*DLQReplayAllResponse, error) {
	u, _ := url.Parse(c.server + "/api/v1/dlq/replay-all")
	if topic != "" {
		q := u.Query()
		q.Set("topic", topic)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest("POST", u.String(), nil)
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
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to replay all"}
	}

	var result DLQReplayAllResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DLQPurgeResponse is the response from purge.
type DLQPurgeResponse struct {
	Deleted int `json:"deleted"`
}

// DLQPurge deletes all messages from the DLQ.
func (c *Client) DLQPurge(topic string) (*DLQPurgeResponse, error) {
	u, _ := url.Parse(c.server + "/api/v1/dlq/purge")
	if topic != "" {
		q := u.Query()
		q.Set("topic", topic)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest("DELETE", u.String(), nil)
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
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to purge DLQ"}
	}

	var result DLQPurgeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

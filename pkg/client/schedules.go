package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ScheduleRequest is the request body for creating a scheduled event.
type ScheduleRequest struct {
	Topic        string          `json:"topic"`
	Data         json.RawMessage `json:"data"`
	ScheduledFor *time.Time      `json:"scheduled_for,omitempty"`
	In           string          `json:"in,omitempty"`
}

// ScheduleResponse is the response body for a scheduled event.
type ScheduleResponse struct {
	ID           string          `json:"id"`
	Topic        string          `json:"topic"`
	Data         json.RawMessage `json:"data,omitempty"`
	ScheduledFor time.Time       `json:"scheduled_for"`
	Status       string          `json:"status,omitempty"`
	Error        *string         `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	ExecutedAt   *time.Time      `json:"executed_at,omitempty"`
}

// SchedulesListResponse is the response body for listing scheduled events.
type SchedulesListResponse struct {
	Schedules []ScheduleResponse `json:"schedules"`
	Count     int                `json:"count"`
}

// RunScheduleResponse is the response body for executing a scheduled event.
type RunScheduleResponse struct {
	ScheduleID string `json:"schedule_id"`
	EventID    string `json:"event_id"`
}

// Schedule creates a new scheduled event.
func (c *Client) Schedule(topic string, data json.RawMessage, scheduledFor *time.Time, in string) (*ScheduleResponse, error) {
	req := ScheduleRequest{
		Topic:        topic,
		Data:         data,
		ScheduledFor: scheduledFor,
		In:           in,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.server+"/api/v1/schedules", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var result ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListSchedules lists scheduled events.
func (c *Client) ListSchedules(status string, limit, offset int) (*SchedulesListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/schedules?limit=%d&offset=%d", c.server, limit, offset)
	if status != "" {
		url += "&status=" + status
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var result SchedulesListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetSchedule gets a scheduled event by ID.
func (c *Client) GetSchedule(id string) (*ScheduleResponse, error) {
	httpReq, err := http.NewRequest("GET", c.server+"/api/v1/schedules/"+id, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var result ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CancelSchedule cancels a scheduled event.
func (c *Client) CancelSchedule(id string) error {
	httpReq, err := http.NewRequest("DELETE", c.server+"/api/v1/schedules/"+id, nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	return nil
}

// RunSchedule executes a scheduled event immediately.
func (c *Client) RunSchedule(id string) (*RunScheduleResponse, error) {
	httpReq, err := http.NewRequest("POST", c.server+"/api/v1/schedules/"+id+"/run", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var result RunScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

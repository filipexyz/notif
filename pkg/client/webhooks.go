package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Webhook represents a webhook configuration.
type Webhook struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Topics      []string `json:"topics"`
	Secret      string   `json:"secret,omitempty"`
	Enabled     bool     `json:"enabled"`
	Environment string   `json:"environment"`
	CreatedAt   string   `json:"created_at"`
}

// WebhookListResponse is the response from listing webhooks.
type WebhookListResponse struct {
	Webhooks []Webhook `json:"webhooks"`
	Count    int       `json:"count"`
}

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID             string     `json:"id"`
	WebhookID      string     `json:"webhook_id"`
	EventID        string     `json:"event_id"`
	Topic          string     `json:"topic"`
	Status         string     `json:"status"`
	Attempt        int        `json:"attempt"`
	ResponseStatus *int       `json:"response_status"`
	ResponseBody   *string    `json:"response_body"`
	Error          *string    `json:"error"`
	CreatedAt      time.Time  `json:"created_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
}

// WebhookDeliveriesResponse is the response from listing deliveries.
type WebhookDeliveriesResponse struct {
	Deliveries []WebhookDelivery `json:"deliveries"`
	Count      int               `json:"count"`
}

// CreateWebhookRequest is the request to create a webhook.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Topics []string `json:"topics"`
}

// WebhookCreate creates a new webhook.
func (c *Client) WebhookCreate(url string, topics []string) (*Webhook, error) {
	reqBody, _ := json.Marshal(CreateWebhookRequest{
		URL:    url,
		Topics: topics,
	})

	req, err := http.NewRequest("POST", c.server+"/webhooks", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Message: "invalid or missing API key"}
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var webhook Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhook); err != nil {
		return nil, err
	}

	return &webhook, nil
}

// WebhookList lists all webhooks.
func (c *Client) WebhookList() (*WebhookListResponse, error) {
	req, err := http.NewRequest("GET", c.server+"/webhooks", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to list webhooks"}
	}

	var result WebhookListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// WebhookGet retrieves a specific webhook.
func (c *Client) WebhookGet(id string) (*Webhook, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/webhooks/%s", c.server, id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "webhook not found"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get webhook"}
	}

	var webhook Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhook); err != nil {
		return nil, err
	}

	return &webhook, nil
}

// UpdateWebhookRequest is the request to update a webhook.
type UpdateWebhookRequest struct {
	URL     string   `json:"url,omitempty"`
	Topics  []string `json:"topics,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// WebhookUpdate updates a webhook.
func (c *Client) WebhookUpdate(id string, req UpdateWebhookRequest) (*Webhook, error) {
	reqBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/webhooks/%s", c.server, id), bytes.NewReader(reqBody))
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

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to update webhook"}
	}

	var webhook Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhook); err != nil {
		return nil, err
	}

	return &webhook, nil
}

// WebhookDelete deletes a webhook.
func (c *Client) WebhookDelete(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/webhooks/%s", c.server, id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Message: "failed to delete webhook"}
	}

	return nil
}

// WebhookDeliveries lists recent deliveries for a webhook.
func (c *Client) WebhookDeliveries(id string) (*WebhookDeliveriesResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/webhooks/%s/deliveries", c.server, id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get deliveries"}
	}

	var result WebhookDeliveriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultServer  = "https://api.notif.sh"
	DefaultTimeout = 30 * time.Second
)

// Client is the notif.sh API client.
type Client struct {
	apiKey     string
	server     string
	projectID  string // For JWT auth - sent as X-Project-ID header
	httpClient *http.Client
}

// Option configures the client.
type Option func(*Client)

// New creates a new notif.sh client.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey: apiKey,
		server: DefaultServer,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithServer sets a custom server URL.
func WithServer(server string) Option {
	return func(c *Client) {
		if server != "" {
			c.server = server
		}
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithProjectID sets the project ID for JWT auth (sent as X-Project-ID header).
func WithProjectID(projectID string) Option {
	return func(c *Client) {
		c.projectID = projectID
	}
}

// ServerURL returns the configured server URL.
func (c *Client) ServerURL() string {
	return c.server
}

// setAuthHeaders sets authorization and project headers on a request.
func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if c.projectID != "" {
		req.Header.Set("X-Project-ID", c.projectID)
	}
}

// Get performs a GET request and returns the response body.
func (c *Client) Get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil)
}

// Post performs a POST request with a JSON body and returns the response body.
func (c *Client) Post(path string, body []byte) ([]byte, error) {
	return c.doRequest("POST", path, body)
}

// Put performs a PUT request with a JSON body and returns the response body.
func (c *Client) Put(path string, body []byte) ([]byte, error) {
	return c.doRequest("PUT", path, body)
}

// Delete performs a DELETE request and returns the response body.
func (c *Client) Delete(path string) ([]byte, error) {
	return c.doRequest("DELETE", path, nil)
}

func (c *Client) doRequest(method, path string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.server+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setAuthHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

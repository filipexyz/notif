package client

import (
	"net/http"
	"time"
)

const (
	DefaultServer  = "http://localhost:8080"
	DefaultTimeout = 30 * time.Second
)

// Client is the notif.sh API client.
type Client struct {
	apiKey     string
	server     string
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

// ServerURL returns the configured server URL.
func (c *Client) ServerURL() string {
	return c.server
}

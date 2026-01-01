package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	StreamName         = "NOTIF_EVENTS"
	DLQStreamName      = "NOTIF_DLQ"
	WebhookRetryStream = "NOTIF_WEBHOOK_RETRY"
)

// Client wraps NATS connection and JetStream.
type Client struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
}

// Connect establishes a connection to NATS and initializes JetStream.
func Connect(url string) (*Client, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("NATS reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	return &Client{
		conn: nc,
		js:   js,
	}, nil
}

// EnsureStreams creates or updates the required JetStream streams.
func (c *Client) EnsureStreams(ctx context.Context) error {
	// Main events stream
	stream, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        StreamName,
		Description: "notif.sh event stream",
		Subjects:    []string{"events.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      24 * time.Hour,
		MaxBytes:    1 << 30, // 1GB
		Replicas:    1,
		Discard:     jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("create events stream: %w", err)
	}
	c.stream = stream
	slog.Info("JetStream stream ready", "name", StreamName)

	// Dead letter queue stream
	_, err = c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        DLQStreamName,
		Description: "notif.sh dead letter queue",
		Subjects:    []string{"dlq.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		Replicas:    1,
	})
	if err != nil {
		return fmt.Errorf("create DLQ stream: %w", err)
	}
	slog.Info("JetStream stream ready", "name", DLQStreamName)

	// Webhook retry stream
	_, err = c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        WebhookRetryStream,
		Description: "notif.sh webhook retry queue",
		Subjects:    []string{"webhook-retry.>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})
	if err != nil {
		return fmt.Errorf("create webhook retry stream: %w", err)
	}
	slog.Info("JetStream stream ready", "name", WebhookRetryStream)

	return nil
}

// JetStream returns the JetStream context.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Stream returns the main events stream.
func (c *Client) Stream() jetstream.Stream {
	return c.stream
}

// Close closes the NATS connection.
func (c *Client) Close() {
	c.conn.Drain()
}

// IsConnected returns true if connected to NATS.
func (c *Client) IsConnected() bool {
	return c.conn.IsConnected()
}

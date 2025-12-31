package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024
)

// Client represents a WebSocket client connection.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	apiKeyID string
	env      string

	// Subscription state
	mu              sync.RWMutex
	consumer        jetstream.Consumer
	consumerContext jetstream.ConsumeContext
	pendingMessages map[string]jetstream.Msg
	autoAck         bool
	maxRetries      int
}

// NewClient creates a new WebSocket client.
func NewClient(hub *Hub, conn *websocket.Conn, apiKeyID, env string) *Client {
	return &Client{
		hub:             hub,
		conn:            conn,
		send:            make(chan []byte, 256),
		apiKeyID:        apiKeyID,
		env:             env,
		pendingMessages: make(map[string]jetstream.Msg),
		maxRetries:      5,
	}
}

// ReadPump reads messages from the WebSocket connection.
func (c *Client) ReadPump(ctx context.Context, consumerMgr *nats.ConsumerManager) {
	defer func() {
		c.cleanup()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("websocket read error", "error", err)
			}
			return
		}

		c.handleMessage(ctx, message, consumerMgr)
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(ctx context.Context, data []byte, consumerMgr *nats.ConsumerManager) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("INVALID_JSON", "invalid JSON message")
		return
	}

	switch msg.Action {
	case "subscribe":
		var sub SubscribeMessage
		if err := json.Unmarshal(data, &sub); err != nil {
			c.sendError("INVALID_JSON", "invalid subscribe message")
			return
		}
		c.handleSubscribe(ctx, &sub, consumerMgr)

	case "ack":
		var ack AckMessage
		if err := json.Unmarshal(data, &ack); err != nil {
			c.sendError("INVALID_JSON", "invalid ack message")
			return
		}
		c.handleAck(&ack)

	case "nack":
		var nack NackMessage
		if err := json.Unmarshal(data, &nack); err != nil {
			c.sendError("INVALID_JSON", "invalid nack message")
			return
		}
		c.handleNack(&nack)

	case "ping":
		c.sendJSON(NewPongMessage())

	default:
		c.sendError("UNKNOWN_ACTION", "unknown action: "+msg.Action)
	}
}

func (c *Client) handleSubscribe(ctx context.Context, msg *SubscribeMessage, consumerMgr *nats.ConsumerManager) {
	if len(msg.Topics) == 0 {
		c.sendError("INVALID_TOPICS", "at least one topic required")
		return
	}

	// Parse options
	opts := nats.DefaultSubscriptionOptions()
	opts.Topics = msg.Topics
	opts.AutoAck = msg.Options.AutoAck
	opts.Group = msg.Options.Group

	if msg.Options.MaxRetries > 0 {
		opts.MaxRetries = msg.Options.MaxRetries
	}
	if msg.Options.AckTimeout != "" {
		opts.AckTimeout = ParseDuration(msg.Options.AckTimeout)
	}

	c.mu.Lock()
	c.autoAck = opts.AutoAck
	c.maxRetries = opts.MaxRetries
	c.mu.Unlock()

	// Create consumer
	consumer, err := consumerMgr.CreateConsumer(ctx, opts)
	if err != nil {
		slog.Error("failed to create consumer", "error", err)
		c.sendError("CONSUMER_ERROR", "failed to create subscription")
		return
	}

	c.mu.Lock()
	c.consumer = consumer
	c.mu.Unlock()

	// Start consuming
	consCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		c.deliverMessage(msg)
	})
	if err != nil {
		slog.Error("failed to start consuming", "error", err)
		c.sendError("CONSUMER_ERROR", "failed to start subscription")
		return
	}

	c.mu.Lock()
	c.consumerContext = consCtx
	c.mu.Unlock()

	info, _ := consumer.Info(ctx)
	consumerName := ""
	if info != nil {
		consumerName = info.Name
	}

	c.sendJSON(NewSubscribedMessage(msg.Topics, consumerName))
	slog.Info("client subscribed", "topics", msg.Topics, "consumer", consumerName)
}

func (c *Client) deliverMessage(msg jetstream.Msg) {
	// Parse the event from NATS message
	var event domain.Event
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.Error("failed to unmarshal event", "error", err)
		msg.Nak()
		return
	}

	// Get metadata for attempt count
	meta, _ := msg.Metadata()
	attempt := 1
	if meta != nil {
		attempt = int(meta.NumDelivered)
	}

	c.mu.RLock()
	autoAck := c.autoAck
	maxRetries := c.maxRetries
	c.mu.RUnlock()

	// Send to client
	eventMsg := NewEventMessage(event.ID, event.Topic, event.Data, event.Timestamp, attempt, maxRetries)
	c.sendJSON(eventMsg)

	if autoAck {
		msg.Ack()
	} else {
		// Store for manual ack
		c.mu.Lock()
		c.pendingMessages[event.ID] = msg
		c.mu.Unlock()
	}
}

func (c *Client) handleAck(msg *AckMessage) {
	c.mu.Lock()
	natsMsg, ok := c.pendingMessages[msg.ID]
	if ok {
		delete(c.pendingMessages, msg.ID)
	}
	c.mu.Unlock()

	if !ok {
		c.sendError("UNKNOWN_EVENT", "unknown event ID: "+msg.ID)
		return
	}

	if err := natsMsg.Ack(); err != nil {
		slog.Error("failed to ack", "error", err, "event_id", msg.ID)
		c.sendError("ACK_ERROR", "failed to acknowledge")
		return
	}

	slog.Debug("event acked", "event_id", msg.ID)
}

func (c *Client) handleNack(msg *NackMessage) {
	c.mu.Lock()
	natsMsg, ok := c.pendingMessages[msg.ID]
	if ok {
		delete(c.pendingMessages, msg.ID)
	}
	c.mu.Unlock()

	if !ok {
		c.sendError("UNKNOWN_EVENT", "unknown event ID: "+msg.ID)
		return
	}

	delay := ParseDuration(msg.RetryIn)
	if delay == 0 {
		delay = 5 * time.Minute
	}

	if err := natsMsg.NakWithDelay(delay); err != nil {
		slog.Error("failed to nack", "error", err, "event_id", msg.ID)
		c.sendError("NACK_ERROR", "failed to negative acknowledge")
		return
	}

	slog.Debug("event nacked", "event_id", msg.ID, "retry_in", delay)
}

func (c *Client) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.consumerContext != nil {
		c.consumerContext.Stop()
	}

	// Nack any pending messages so they get redelivered
	for _, msg := range c.pendingMessages {
		msg.Nak()
	}
	c.pendingMessages = nil
}

func (c *Client) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal message", "error", err)
		return
	}

	select {
	case c.send <- data:
	default:
		slog.Warn("client send buffer full, dropping message")
	}
}

func (c *Client) sendError(code, message string) {
	c.sendJSON(NewErrorMessage(code, message))
}

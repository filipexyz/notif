package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"
)

// pendingMsg holds a NATS message and its metadata for DLQ handling.
type pendingMsg struct {
	msg        jetstream.Msg
	event      *domain.Event
	attempt    int
	deliveryID pgtype.UUID // Tracks delivery in event_deliveries table
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client represents a WebSocket client connection.
type Client struct {
	hub            *Hub
	conn           *websocket.Conn
	send           chan []byte
	apiKeyID       string
	orgID          string      // Organization ID for multi-tenant isolation
	projectID      string      // Project ID for multi-tenant isolation
	clientID       string      // Unique client identifier for tracking
	queries        *db.Queries // For delivery tracking
	maxMessageSize int64       // Max inbound message size

	// Subscription state
	mu              sync.RWMutex
	consumer        jetstream.Consumer
	consumerContext jetstream.ConsumeContext
	consumerName    string // NATS consumer name for delivery tracking
	pendingMessages map[string]*pendingMsg
	autoAck         bool
	maxRetries      int
	group           string
	dlqPublisher    *nats.DLQPublisher
}

// NewClient creates a new WebSocket client.
func NewClient(hub *Hub, conn *websocket.Conn, apiKeyID, orgID, projectID string, dlqPublisher *nats.DLQPublisher, queries *db.Queries, clientID string, maxMessageSize int64) *Client {
	return &Client{
		hub:             hub,
		conn:            conn,
		send:            make(chan []byte, 256),
		apiKeyID:        apiKeyID,
		orgID:           orgID,
		projectID:       projectID,
		clientID:        clientID,
		queries:         queries,
		pendingMessages: make(map[string]*pendingMsg),
		maxRetries:      5,
		dlqPublisher:    dlqPublisher,
		maxMessageSize:  maxMessageSize,
	}
}

// ReadPump reads messages from the WebSocket connection.
func (c *Client) ReadPump(ctx context.Context, consumerMgr *nats.ConsumerManager) {
	defer func() {
		c.cleanup()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.maxMessageSize)
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

	// OrgID and ProjectID are required for multi-tenant isolation
	if c.orgID == "" {
		c.sendError("UNAUTHORIZED", "org_id is required for subscriptions")
		return
	}
	if c.projectID == "" {
		c.sendError("UNAUTHORIZED", "project_id is required for subscriptions")
		return
	}

	// Parse options
	opts := nats.DefaultSubscriptionOptions()
	opts.Topics = msg.Topics
	opts.OrgID = c.orgID
	opts.ProjectID = c.projectID
	opts.AutoAck = msg.Options.AutoAck
	opts.Group = msg.Options.Group
	opts.From = msg.Options.From

	if msg.Options.MaxRetries > 0 {
		opts.MaxRetries = msg.Options.MaxRetries
	}
	if msg.Options.AckTimeout != "" {
		opts.AckTimeout = ParseDuration(msg.Options.AckTimeout)
	}

	c.mu.Lock()
	c.autoAck = opts.AutoAck
	c.maxRetries = opts.MaxRetries
	c.group = opts.Group
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

	info, _ := consumer.Info(ctx)
	consumerName := ""
	if info != nil {
		consumerName = info.Name
	}

	c.mu.Lock()
	c.consumerContext = consCtx
	c.consumerName = consumerName
	c.mu.Unlock()

	c.sendJSON(NewSubscribedMessage(msg.Topics, consumerName))
	slog.Info("client subscribed", "topics", msg.Topics, "consumer", consumerName, "client_id", c.clientID)
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
	consumerName := c.consumerName
	c.mu.RUnlock()

	// Track delivery in database
	var deliveryID pgtype.UUID
	if c.queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		delivery, err := c.queries.CreateEventDelivery(ctx, db.CreateEventDeliveryParams{
			EventID:      event.ID,
			ReceiverType: "websocket",
			ConsumerName: pgtype.Text{String: consumerName, Valid: consumerName != ""},
			ClientID:     pgtype.Text{String: c.clientID, Valid: c.clientID != ""},
			Status:       "delivered",
			Attempt:      int32(attempt),
			DeliveredAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		cancel()
		if err != nil {
			slog.Warn("failed to track delivery", "error", err, "event_id", event.ID)
		} else {
			deliveryID = delivery.ID
		}
	}

	// Send to client
	eventMsg := NewEventMessage(event.ID, event.Topic, event.Data, event.Timestamp, attempt, maxRetries)
	c.sendJSON(eventMsg)

	if autoAck {
		msg.Ack()
		// Mark delivery as acked immediately for auto-ack mode
		if c.queries != nil && deliveryID.Valid {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.queries.UpdateEventDeliveryAcked(ctx, deliveryID)
			cancel()
		}
	} else {
		// Store for manual ack with metadata for DLQ handling
		c.mu.Lock()
		c.pendingMessages[event.ID] = &pendingMsg{
			msg:        msg,
			event:      &event,
			attempt:    attempt,
			deliveryID: deliveryID,
		}
		c.mu.Unlock()
	}
}

func (c *Client) handleAck(msg *AckMessage) {
	c.mu.Lock()
	pending, ok := c.pendingMessages[msg.ID]
	if ok {
		delete(c.pendingMessages, msg.ID)
	}
	c.mu.Unlock()

	if !ok {
		c.sendError("UNKNOWN_EVENT", "unknown event ID: "+msg.ID)
		return
	}

	if err := pending.msg.Ack(); err != nil {
		slog.Error("failed to ack", "error", err, "event_id", msg.ID)
		c.sendError("ACK_ERROR", "failed to acknowledge")
		return
	}

	// Track ACK in database
	if c.queries != nil && pending.deliveryID.Valid {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		c.queries.UpdateEventDeliveryAcked(ctx, pending.deliveryID)
		cancel()
	}

	slog.Debug("event acked", "event_id", msg.ID)
}

func (c *Client) handleNack(msg *NackMessage) {
	c.mu.Lock()
	pending, ok := c.pendingMessages[msg.ID]
	if ok {
		delete(c.pendingMessages, msg.ID)
	}
	maxRetries := c.maxRetries
	group := c.group
	c.mu.Unlock()

	if !ok {
		c.sendError("UNKNOWN_EVENT", "unknown event ID: "+msg.ID)
		return
	}

	// If at max retries, move to DLQ instead of nacking
	if pending.attempt >= maxRetries {
		c.moveToDLQ(pending, group, "max retries exceeded")
		if err := pending.msg.Term(); err != nil {
			slog.Error("failed to terminate message", "error", err, "event_id", msg.ID)
		}
		// Track DLQ in database
		if c.queries != nil && pending.deliveryID.Valid {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.queries.UpdateEventDeliveryDLQ(ctx, db.UpdateEventDeliveryDLQParams{
				ID:    pending.deliveryID,
				Error: pgtype.Text{String: "max retries exceeded", Valid: true},
			})
			cancel()
		}
		slog.Info("event moved to DLQ", "event_id", msg.ID, "attempts", pending.attempt)
		return
	}

	delay := ParseDuration(msg.RetryIn)
	if delay == 0 {
		delay = 5 * time.Minute
	}

	if err := pending.msg.NakWithDelay(delay); err != nil {
		slog.Error("failed to nack", "error", err, "event_id", msg.ID)
		c.sendError("NACK_ERROR", "failed to negative acknowledge")
		return
	}

	// Track NACK in database
	if c.queries != nil && pending.deliveryID.Valid {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		c.queries.UpdateEventDeliveryNacked(ctx, db.UpdateEventDeliveryNackedParams{
			ID:    pending.deliveryID,
			Error: pgtype.Text{String: "nacked for retry", Valid: true},
		})
		cancel()
	}

	slog.Debug("event nacked", "event_id", msg.ID, "retry_in", delay)
}

func (c *Client) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.consumerContext != nil {
		c.consumerContext.Stop()
	}

	// Handle pending messages - either nack for retry or move to DLQ
	for _, pending := range c.pendingMessages {
		if pending.attempt >= c.maxRetries {
			// At max retries, move to DLQ
			c.moveToDLQ(pending, c.group, "client disconnected at max retries")
			pending.msg.Term()
			// Track DLQ in database
			if c.queries != nil && pending.deliveryID.Valid {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				c.queries.UpdateEventDeliveryDLQ(ctx, db.UpdateEventDeliveryDLQParams{
					ID:    pending.deliveryID,
					Error: pgtype.Text{String: "client disconnected at max retries", Valid: true},
				})
				cancel()
			}
			slog.Info("event moved to DLQ on disconnect", "event_id", pending.event.ID)
		} else {
			// Still has retries left, nack for redelivery
			pending.msg.Nak()
			// Track NACK in database
			if c.queries != nil && pending.deliveryID.Valid {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				c.queries.UpdateEventDeliveryNacked(ctx, db.UpdateEventDeliveryNackedParams{
					ID:    pending.deliveryID,
					Error: pgtype.Text{String: "client disconnected", Valid: true},
				})
				cancel()
			}
		}
	}
	c.pendingMessages = nil
}

func (c *Client) moveToDLQ(pending *pendingMsg, group, reason string) {
	if c.dlqPublisher == nil {
		return
	}

	dlqMsg := &nats.DLQMessage{
		ID:            pending.event.ID,
		OrgID:         c.orgID,
		ProjectID:     c.projectID,
		OriginalTopic: pending.event.Topic,
		Data:          pending.event.Data,
		Timestamp:     pending.event.Timestamp,
		FailedAt:      time.Now().UTC(),
		Attempts:      pending.attempt,
		LastError:     reason,
		ConsumerGroup: group,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.dlqPublisher.Publish(ctx, dlqMsg); err != nil {
		slog.Error("failed to publish to DLQ", "error", err, "event_id", pending.event.ID)
	}
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

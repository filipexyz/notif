package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	maxRetries     = 5
	requestTimeout = 30 * time.Second
)

// Worker handles webhook deliveries.
type Worker struct {
	queries    *db.Queries
	httpClient *http.Client
	stream     jetstream.Stream
}

// NewWorker creates a new webhook worker.
func NewWorker(queries *db.Queries, stream jetstream.Stream) *Worker {
	return &Worker{
		queries: queries,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		stream: stream,
	}
}

// Start begins processing events for webhook delivery.
func (w *Worker) Start(ctx context.Context) error {
	// Create a consumer for all events
	consumer, err := w.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "webhook-worker",
		FilterSubject: "events.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       time.Minute,
		MaxDeliver:    maxRetries,
	})
	if err != nil {
		return fmt.Errorf("create webhook consumer: %w", err)
	}

	// Start consuming
	consCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		w.processMessage(ctx, msg)
	})
	if err != nil {
		return fmt.Errorf("start webhook consumer: %w", err)
	}

	slog.Info("webhook worker started")

	// Wait for context cancellation
	<-ctx.Done()
	consCtx.Stop()

	return nil
}

func (w *Worker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var event domain.Event
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.Error("webhook: failed to unmarshal event", "error", err)
		msg.Ack() // Don't retry malformed messages
		return
	}

	// Get enabled webhooks
	// TODO: Cache webhooks and refresh periodically
	webhooks, err := w.queries.GetEnabledWebhooks(ctx, "test") // TODO: Get environment from event
	if err != nil {
		slog.Error("webhook: failed to get webhooks", "error", err)
		msg.NakWithDelay(time.Minute)
		return
	}

	// Find matching webhooks
	delivered := 0
	for _, wh := range webhooks {
		if !matchesTopic(wh.Topics, event.Topic) {
			continue
		}

		if err := w.deliver(ctx, &wh, &event); err != nil {
			slog.Error("webhook: delivery failed",
				"webhook_id", wh.ID,
				"event_id", event.ID,
				"error", err,
			)
		} else {
			delivered++
		}
	}

	if delivered > 0 {
		slog.Debug("webhook: delivered event",
			"event_id", event.ID,
			"topic", event.Topic,
			"webhooks", delivered,
		)
	}

	msg.Ack()
}

func (w *Worker) deliver(ctx context.Context, wh *db.Webhook, event *domain.Event) error {
	// Build payload
	payload := WebhookPayload{
		ID:        event.ID,
		Topic:     event.Topic,
		Data:      event.Data,
		Timestamp: event.Timestamp,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Create signature
	signature := sign(body, wh.Secret)

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", wh.Url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Notif-Signature", signature)
	req.Header.Set("X-Notif-Event-ID", event.ID)
	req.Header.Set("X-Notif-Topic", event.Topic)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		// Record failed delivery
		w.recordDelivery(ctx, wh.ID, event.ID, event.Topic, 0, "", err.Error())
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.recordDelivery(ctx, wh.ID, event.ID, event.Topic, resp.StatusCode, string(respBody), "")
		return nil
	}

	errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
	w.recordDelivery(ctx, wh.ID, event.ID, event.Topic, resp.StatusCode, string(respBody), errMsg)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
}

func (w *Worker) recordDelivery(ctx context.Context, webhookID pgtype.UUID, eventID, topic string, statusCode int, respBody, errMsg string) {
	delivery, err := w.queries.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
		WebhookID: webhookID,
		EventID:   eventID,
		Topic:     topic,
	})
	if err != nil {
		slog.Error("webhook: failed to record delivery", "error", err)
		return
	}

	// Update with result
	status := "success"
	if errMsg != "" {
		status = "failed"
	}

	var respStatus pgtype.Int4
	if statusCode > 0 {
		respStatus = pgtype.Int4{Int32: int32(statusCode), Valid: true}
	}

	var respBodyText pgtype.Text
	if respBody != "" {
		respBodyText = pgtype.Text{String: respBody, Valid: true}
	}

	var errorText pgtype.Text
	if errMsg != "" {
		errorText = pgtype.Text{String: errMsg, Valid: true}
	}

	var deliveredAt pgtype.Timestamptz
	if status == "success" {
		deliveredAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
		ID:             delivery.ID,
		Status:         status,
		Attempt:        1,
		ResponseStatus: respStatus,
		ResponseBody:   respBodyText,
		Error:          errorText,
		DeliveredAt:    deliveredAt,
	})
}

// WebhookPayload is the payload sent to webhook endpoints.
type WebhookPayload struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// sign creates an HMAC-SHA256 signature.
func sign(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// matchesTopic checks if an event topic matches any of the webhook patterns.
func matchesTopic(patterns []string, topic string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, topic) {
			return true
		}
	}
	return false
}

// matchPattern checks if a topic matches a pattern (supports * and > wildcards).
func matchPattern(pattern, topic string) bool {
	if pattern == ">" || pattern == "*" {
		return true
	}

	patternParts := strings.Split(pattern, ".")
	topicParts := strings.Split(topic, ".")

	for i, pp := range patternParts {
		if pp == ">" {
			return true // > matches all remaining segments
		}
		if i >= len(topicParts) {
			return false
		}
		if pp == "*" {
			continue // * matches one segment
		}
		if pp != topicParts[i] {
			return false
		}
	}

	return len(patternParts) == len(topicParts)
}

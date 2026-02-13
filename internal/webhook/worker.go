package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	notifnats "github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/security"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	maxRetries     = 5
	requestTimeout = 30 * time.Second
)

// retryDelays defines exponential backoff delays for retries
var retryDelays = []time.Duration{
	10 * time.Second,  // 1st retry
	30 * time.Second,  // 2nd retry
	2 * time.Minute,   // 3rd retry
	10 * time.Minute,  // 4th retry
	30 * time.Minute,  // 5th retry
}

// RetryJob represents a webhook delivery retry job.
// Note: Secret and URL are fetched from the database at retry time
// instead of being stored in the message queue.
type RetryJob struct {
	WebhookID  string          `json:"webhook_id"`
	EventID    string          `json:"event_id"`
	OrgID      string          `json:"org_id"`
	Topic      string          `json:"topic"`
	Data       json.RawMessage `json:"data"`
	Timestamp  time.Time       `json:"timestamp"`
	Attempt    int             `json:"attempt"`
	LastError  string          `json:"last_error"`
	DeliveryID string          `json:"delivery_id"`
}

// Worker handles webhook deliveries.
type Worker struct {
	queries      *db.Queries
	httpClient   *http.Client
	stream       jetstream.Stream
	js           jetstream.JetStream
	dlqPublisher *notifnats.DLQPublisher
}

// NewWorker creates a new webhook worker.
func NewWorker(queries *db.Queries, stream jetstream.Stream, js jetstream.JetStream, dlqPublisher *notifnats.DLQPublisher) *Worker {
	return &Worker{
		queries:      queries,
		httpClient:   newSafeHTTPClient(),
		stream:       stream,
		js:           js,
		dlqPublisher: dlqPublisher,
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
		MaxDeliver:    1, // We handle retries ourselves via retry queue
	})
	if err != nil {
		return fmt.Errorf("create webhook consumer: %w", err)
	}

	// Start consuming events
	consCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		w.processMessage(ctx, msg)
	})
	if err != nil {
		return fmt.Errorf("start webhook consumer: %w", err)
	}

	// Start retry consumer
	go w.startRetryConsumer(ctx)

	slog.Info("webhook worker started")

	// Wait for context cancellation
	<-ctx.Done()
	consCtx.Stop()

	return nil
}

// startRetryConsumer consumes from the webhook retry queue
func (w *Worker) startRetryConsumer(ctx context.Context) {
	retryStream, err := w.js.Stream(ctx, notifnats.WebhookRetryStream)
	if err != nil {
		slog.Error("failed to get retry stream", "error", err)
		return
	}

	consumer, err := retryStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:    "webhook-retry-worker",
		AckPolicy:  jetstream.AckExplicitPolicy,
		AckWait:    2 * time.Minute,
		MaxDeliver: 1, // We track retries in the job itself
	})
	if err != nil {
		slog.Error("failed to create retry consumer", "error", err)
		return
	}

	consCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		w.processRetry(ctx, msg)
	})
	if err != nil {
		slog.Error("failed to start retry consumer", "error", err)
		return
	}

	slog.Info("webhook retry worker started")

	<-ctx.Done()
	consCtx.Stop()
}

func (w *Worker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var event domain.Event
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.Error("webhook: failed to unmarshal event", "error", err)
		msg.Ack() // Don't retry malformed messages
		return
	}

	// Get webhooks for this org
	if event.OrgID == "" {
		slog.Warn("webhook: event has no org_id, skipping", "event_id", event.ID)
		msg.Ack()
		return
	}

	webhooks, err := w.queries.GetEnabledWebhooksByOrg(ctx, pgtype.Text{String: event.OrgID, Valid: true})
	if err != nil {
		slog.Error("webhook: failed to get webhooks", "error", err)
		msg.NakWithDelay(time.Minute)
		return
	}

	// Find matching webhooks and attempt delivery
	for _, wh := range webhooks {
		if !matchesTopic(wh.Topics, event.Topic) {
			continue
		}

		// Create delivery record
		delivery, err := w.queries.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
			WebhookID: wh.ID,
			EventID:   event.ID,
			Topic:     event.Topic,
		})
		if err != nil {
			slog.Error("webhook: failed to create delivery record", "error", err)
			continue
		}

		deliveryID := pgUUIDToString(delivery.ID)

		// Attempt delivery
		errMsg := w.deliver(ctx, &wh, &event)
		if errMsg == "" {
			// Success
			w.updateDeliverySuccess(ctx, delivery.ID)
			w.recordEventDelivery(ctx, wh.ID, event.ID, "acked", 1)
			slog.Debug("webhook: delivered event", "event_id", event.ID, "webhook_id", pgUUIDToString(wh.ID))
		} else {
			// Failed - schedule retry
			w.updateDeliveryFailed(ctx, delivery.ID, 1, errMsg)
			w.scheduleRetry(ctx, &wh, &event, 1, errMsg, deliveryID)
		}
	}

	msg.Ack()
}

func (w *Worker) processRetry(ctx context.Context, msg jetstream.Msg) {
	var job RetryJob
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		slog.Error("webhook: failed to unmarshal retry job", "error", err)
		msg.Ack()
		return
	}

	// Fetch webhook from database to get current URL and secret
	webhookID := parseUUID(job.WebhookID)
	dbWebhook, err := w.queries.GetWebhook(ctx, webhookID)
	if err != nil {
		slog.Warn("webhook: webhook not found for retry, skipping", "webhook_id", job.WebhookID)
		msg.Ack()
		return
	}

	wh := &db.Webhook{
		ID:     dbWebhook.ID,
		Url:    dbWebhook.Url,
		Secret: dbWebhook.Secret,
	}

	event := &domain.Event{
		ID:        job.EventID,
		OrgID:     job.OrgID,
		Topic:     job.Topic,
		Data:      job.Data,
		Timestamp: job.Timestamp,
	}

	// Attempt delivery
	errMsg := w.deliver(ctx, wh, event)

	deliveryID := parseUUID(job.DeliveryID)

	if errMsg == "" {
		// Success
		w.updateDeliverySuccess(ctx, deliveryID)
		w.recordEventDelivery(ctx, parseUUID(job.WebhookID), event.ID, "acked", int32(job.Attempt))
		slog.Info("webhook: retry succeeded", "event_id", event.ID, "attempt", job.Attempt)
	} else {
		// Failed
		w.updateDeliveryFailed(ctx, deliveryID, int32(job.Attempt), errMsg)

		if job.Attempt >= maxRetries {
			// Max retries reached - move to DLQ
			w.moveToDLQ(ctx, &job, errMsg)
			w.recordEventDelivery(ctx, parseUUID(job.WebhookID), event.ID, "dlq", int32(job.Attempt))
			slog.Warn("webhook: max retries reached, moved to DLQ",
				"event_id", event.ID,
				"webhook_id", job.WebhookID,
				"attempts", job.Attempt,
			)
		} else {
			// Schedule next retry
			job.Attempt++
			job.LastError = errMsg
			w.publishRetryJob(ctx, &job)
		}
	}

	msg.Ack()
}

func (w *Worker) deliver(ctx context.Context, wh *db.Webhook, event *domain.Event) string {
	// Build payload
	payload := WebhookPayload{
		ID:        event.ID,
		Topic:     event.Topic,
		Data:      event.Data,
		Timestamp: event.Timestamp,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("marshal payload: %v", err)
	}

	// Create signature
	signature := sign(body, wh.Secret)

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", wh.Url, bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Notif-Signature", signature)
	req.Header.Set("X-Notif-Event-ID", event.ID)
	req.Header.Set("X-Notif-Topic", event.Topic)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "" // Success
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
}

func (w *Worker) scheduleRetry(ctx context.Context, wh *db.Webhook, event *domain.Event, attempt int, lastError, deliveryID string) {
	job := &RetryJob{
		WebhookID:  pgUUIDToString(wh.ID),
		EventID:    event.ID,
		OrgID:      event.OrgID,
		Topic:      event.Topic,
		Data:       event.Data,
		Timestamp:  event.Timestamp,
		Attempt:    attempt + 1,
		LastError:  lastError,
		DeliveryID: deliveryID,
	}

	w.publishRetryJob(ctx, job)
}

func (w *Worker) publishRetryJob(ctx context.Context, job *RetryJob) {
	data, err := json.Marshal(job)
	if err != nil {
		slog.Error("webhook: failed to marshal retry job", "error", err)
		return
	}

	// Calculate delay based on attempt number
	delay := retryDelays[0]
	if job.Attempt-1 < len(retryDelays) {
		delay = retryDelays[job.Attempt-1]
	}

	subject := fmt.Sprintf("webhook-retry.%s.%s", job.OrgID, job.WebhookID)

	// Publish with headers (NATS doesn't support native delay, so we'll use AckWait on consumer)
	// For now, use a simple approach: publish immediately and the retry consumer picks it up
	// In production, you might want a more sophisticated delay queue
	go func() {
		time.Sleep(delay)
		if _, err := w.js.Publish(ctx, subject, data); err != nil {
			slog.Error("webhook: failed to publish retry job", "error", err, "event_id", job.EventID)
		} else {
			slog.Debug("webhook: scheduled retry", "event_id", job.EventID, "attempt", job.Attempt, "delay", delay)
		}
	}()
}

func (w *Worker) moveToDLQ(ctx context.Context, job *RetryJob, lastError string) {
	if w.dlqPublisher == nil {
		slog.Warn("webhook: DLQ publisher not configured")
		return
	}

	dlqMsg := &notifnats.DLQMessage{
		ID:            job.EventID,
		OrgID:         job.OrgID,
		OriginalTopic: job.Topic,
		Data:          job.Data,
		Timestamp:     job.Timestamp,
		FailedAt:      time.Now(),
		Attempts:      job.Attempt,
		LastError:     fmt.Sprintf("webhook %s: %s", job.WebhookID, lastError),
		ConsumerGroup: "webhook:" + job.WebhookID,
	}

	if err := w.dlqPublisher.Publish(ctx, dlqMsg); err != nil {
		slog.Error("webhook: failed to publish to DLQ", "error", err, "event_id", job.EventID)
	}
}

func (w *Worker) updateDeliverySuccess(ctx context.Context, deliveryID pgtype.UUID) {
	now := time.Now()
	w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
		ID:          deliveryID,
		Status:      "success",
		Attempt:     1,
		DeliveredAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
}

func (w *Worker) updateDeliveryFailed(ctx context.Context, deliveryID pgtype.UUID, attempt int32, errMsg string) {
	w.queries.UpdateWebhookDelivery(ctx, db.UpdateWebhookDeliveryParams{
		ID:      deliveryID,
		Status:  "failed",
		Attempt: attempt,
		Error:   pgtype.Text{String: errMsg, Valid: true},
	})
}

func (w *Worker) recordEventDelivery(ctx context.Context, webhookID pgtype.UUID, eventID, status string, attempt int32) {
	now := time.Now()
	var deliveredAt pgtype.Timestamptz
	if status == "acked" {
		deliveredAt = pgtype.Timestamptz{Time: now, Valid: true}
	}

	_, err := w.queries.CreateEventDelivery(ctx, db.CreateEventDeliveryParams{
		EventID:      eventID,
		ReceiverType: "webhook",
		ReceiverID:   webhookID,
		Status:       status,
		Attempt:      attempt,
		DeliveredAt:  deliveredAt,
	})
	if err != nil {
		slog.Warn("webhook: failed to create event delivery", "error", err, "event_id", eventID)
	}
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

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}

// newSafeHTTPClient creates an HTTP client that validates destination IPs
// on every connection (including redirects) to prevent SSRF attacks.
func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve %s: %w", host, err)
			}
			for _, ip := range ips {
				if err := security.ValidateIP(ip); err != nil {
					return nil, fmt.Errorf("blocked destination %s (%s): %w", host, ip, err)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if s == "" {
		return u
	}
	// Simple hex parsing (assumes valid UUID format)
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return u
	}
	for i := 0; i < 16; i++ {
		fmt.Sscanf(s[i*2:i*2+2], "%02x", &u.Bytes[i])
	}
	u.Valid = true
	return u
}

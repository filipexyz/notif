package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/filipexyz/notif/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
)

// Publisher publishes events to JetStream.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher creates a new Publisher.
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{js: js}
}

// Publish sends an event to JetStream.
func (p *Publisher) Publish(ctx context.Context, event *domain.Event) error {
	// Strict org_id enforcement - no anonymous events allowed
	if event.OrgID == "" {
		return fmt.Errorf("org_id is required for publishing events")
	}

	// Subject format: events.{org_id}.{topic}
	subject := "events." + event.OrgID + "." + event.Topic

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Synchronous publish with ack from JetStream
	ack, err := p.js.Publish(ctx, subject, data,
		jetstream.WithMsgID(event.ID), // Deduplication
	)
	if err != nil {
		return fmt.Errorf("publish to JetStream: %w", err)
	}

	slog.Debug("event published",
		"event_id", event.ID,
		"topic", event.Topic,
		"org_id", event.OrgID,
		"stream", ack.Stream,
		"seq", ack.Sequence,
	)

	return nil
}

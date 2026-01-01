package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filipexyz/notif/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
)

// EventReader reads historical events from the stream.
type EventReader struct {
	stream jetstream.Stream
}

// NewEventReader creates a new EventReader.
func NewEventReader(stream jetstream.Stream) *EventReader {
	return &EventReader{stream: stream}
}

// QueryOptions configures event queries.
type QueryOptions struct {
	Topic string
	OrgID string    // Required: filter by organization
	From  time.Time // Start time (inclusive)
	To    time.Time // End time (exclusive), zero means now
	Limit int
}

// StoredEvent represents an event with its stream metadata.
type StoredEvent struct {
	Seq       uint64        `json:"seq"`
	Event     *domain.Event `json:"event"`
	Timestamp time.Time     `json:"timestamp"`
}

// Query retrieves events matching the options (filtered by org).
func (r *EventReader) Query(ctx context.Context, opts QueryOptions) ([]StoredEvent, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}

	// OrgID is required for multi-tenant isolation
	if opts.OrgID == "" {
		return nil, fmt.Errorf("org_id is required for event queries")
	}

	// Build filter subject with org isolation: events.{org_id}.{topic}
	var filterSubject string
	if opts.Topic != "" {
		filterSubject = "events." + opts.OrgID + "." + opts.Topic
	} else {
		filterSubject = "events." + opts.OrgID + ".>"
	}

	// Create consumer config based on time range
	consumerCfg := jetstream.ConsumerConfig{
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckNonePolicy,
	}

	if !opts.From.IsZero() {
		consumerCfg.DeliverPolicy = jetstream.DeliverByStartTimePolicy
		consumerCfg.OptStartTime = &opts.From
	} else {
		consumerCfg.DeliverPolicy = jetstream.DeliverAllPolicy
	}

	consumer, err := r.stream.CreateOrUpdateConsumer(ctx, consumerCfg)
	if err != nil {
		return nil, err
	}

	events := make([]StoredEvent, 0, opts.Limit)

	// Fetch messages
	msgs, err := consumer.Fetch(opts.Limit, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		return events, nil // No messages or timeout
	}

	for msg := range msgs.Messages() {
		var event domain.Event
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			continue
		}

		meta, _ := msg.Metadata()
		seq := uint64(0)
		msgTime := event.Timestamp
		if meta != nil {
			seq = meta.Sequence.Stream
			msgTime = meta.Timestamp
		}

		// Check time bounds
		if !opts.To.IsZero() && msgTime.After(opts.To) {
			break
		}

		events = append(events, StoredEvent{
			Seq:       seq,
			Event:     &event,
			Timestamp: msgTime,
		})

		if len(events) >= opts.Limit {
			break
		}
	}

	return events, nil
}

// GetBySeq retrieves a specific event by sequence number.
func (r *EventReader) GetBySeq(ctx context.Context, seq uint64) (*StoredEvent, error) {
	msg, err := r.stream.GetMsg(ctx, seq)
	if err != nil {
		return nil, err
	}

	var event domain.Event
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil, err
	}

	return &StoredEvent{
		Seq:       seq,
		Event:     &event,
		Timestamp: msg.Time,
	}, nil
}

// StreamInfo returns information about the events stream.
func (r *EventReader) StreamInfo(ctx context.Context) (*jetstream.StreamInfo, error) {
	return r.stream.Info(ctx)
}

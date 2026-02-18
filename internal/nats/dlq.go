package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// DLQMessage represents a message in the dead letter queue.
type DLQMessage struct {
	ID            string          `json:"id"`
	OrgID         string          `json:"org_id"`
	ProjectID     string          `json:"project_id"`
	OriginalTopic string          `json:"original_topic"`
	Data          json.RawMessage `json:"data"`
	Timestamp     time.Time       `json:"timestamp"`
	FailedAt      time.Time       `json:"failed_at"`
	Attempts      int             `json:"attempts"`
	LastError     string          `json:"last_error,omitempty"`
	ConsumerGroup string          `json:"consumer_group,omitempty"`
}

// DLQPublisher publishes failed messages to the dead letter queue.
type DLQPublisher struct {
	js jetstream.JetStream
}

// NewDLQPublisher creates a new DLQPublisher.
func NewDLQPublisher(js jetstream.JetStream) *DLQPublisher {
	return &DLQPublisher{js: js}
}

// Publish sends a failed message to the DLQ.
func (p *DLQPublisher) Publish(ctx context.Context, msg *DLQMessage) error {
	// OrgID and ProjectID are required for multi-tenant isolation
	if msg.OrgID == "" {
		return fmt.Errorf("org_id is required for DLQ messages")
	}
	if msg.ProjectID == "" {
		return fmt.Errorf("project_id is required for DLQ messages")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal DLQ message: %w", err)
	}

	// Subject format: dlq.{org_id}.{project_id}.<original_topic>
	subject := "dlq." + msg.OrgID + "." + msg.ProjectID + "." + msg.OriginalTopic

	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publish to DLQ: %w", err)
	}

	return nil
}

// DLQReader reads messages from the dead letter queue.
type DLQReader struct {
	js     jetstream.JetStream
	stream jetstream.Stream
}

// NewDLQReader creates a new DLQReader.
func NewDLQReader(js jetstream.JetStream) (*DLQReader, error) {
	stream, err := js.Stream(context.Background(), DLQStreamName)
	if err != nil {
		return nil, fmt.Errorf("get DLQ stream: %w", err)
	}
	return &DLQReader{js: js, stream: stream}, nil
}

// NewDLQReaderForOrg creates a DLQReader for an org-specific DLQ stream.
func NewDLQReaderForOrg(js jetstream.JetStream, orgID string) (*DLQReader, error) {
	streamName := DLQStreamName + "_" + orgID
	stream, err := js.Stream(context.Background(), streamName)
	if err != nil {
		return nil, fmt.Errorf("get DLQ stream for org %s: %w", orgID, err)
	}
	return &DLQReader{js: js, stream: stream}, nil
}

// DLQEntry represents a DLQ message with its sequence number.
type DLQEntry struct {
	Seq     uint64      `json:"seq"`
	Subject string      `json:"subject"`
	Message *DLQMessage `json:"message"`
}

// List returns messages from the DLQ, filtered by org, project, and optionally by topic.
func (r *DLQReader) List(ctx context.Context, orgID, projectID, topic string, limit int) ([]DLQEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	// OrgID and ProjectID are required for multi-tenant isolation
	if orgID == "" {
		return nil, fmt.Errorf("org_id is required for DLQ queries")
	}
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required for DLQ queries")
	}

	// Create ephemeral consumer to read messages with org and project filtering
	// Subject format: dlq.{org_id}.{project_id}.{topic}
	var filterSubject string
	if topic != "" {
		filterSubject = "dlq." + orgID + "." + projectID + "." + topic
	} else {
		filterSubject = "dlq." + orgID + "." + projectID + ".>"
	}

	consumer, err := r.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckNonePolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("create DLQ consumer: %w", err)
	}

	entries := make([]DLQEntry, 0, limit)

	// Fetch messages
	msgs, err := consumer.Fetch(limit, jetstream.FetchMaxWait(time.Second))
	if err != nil {
		return entries, nil // No messages or timeout
	}

	for msg := range msgs.Messages() {
		var dlqMsg DLQMessage
		if err := json.Unmarshal(msg.Data(), &dlqMsg); err != nil {
			continue
		}

		meta, _ := msg.Metadata()
		seq := uint64(0)
		if meta != nil {
			seq = meta.Sequence.Stream
		}

		entries = append(entries, DLQEntry{
			Seq:     seq,
			Subject: msg.Subject(),
			Message: &dlqMsg,
		})
	}

	return entries, nil
}

// Get retrieves a specific DLQ message by sequence number.
func (r *DLQReader) Get(ctx context.Context, seq uint64) (*DLQEntry, error) {
	msg, err := r.stream.GetMsg(ctx, seq)
	if err != nil {
		return nil, fmt.Errorf("get DLQ message: %w", err)
	}

	var dlqMsg DLQMessage
	if err := json.Unmarshal(msg.Data, &dlqMsg); err != nil {
		return nil, fmt.Errorf("unmarshal DLQ message: %w", err)
	}

	return &DLQEntry{
		Seq:     seq,
		Subject: msg.Subject,
		Message: &dlqMsg,
	}, nil
}

// Delete removes a message from the DLQ.
func (r *DLQReader) Delete(ctx context.Context, seq uint64) error {
	return r.stream.DeleteMsg(ctx, seq)
}

// Count returns the total number of messages in the DLQ for a specific org and project.
func (r *DLQReader) Count(ctx context.Context, orgID, projectID string) (int64, error) {
	if orgID == "" {
		return 0, fmt.Errorf("org_id is required for DLQ count")
	}
	if projectID == "" {
		return 0, fmt.Errorf("project_id is required for DLQ count")
	}

	// Create ephemeral consumer to count messages for this org and project
	filterSubject := "dlq." + orgID + "." + projectID + ".>"

	consumer, err := r.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckNonePolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return 0, fmt.Errorf("create DLQ consumer: %w", err)
	}

	info, err := consumer.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("get consumer info: %w", err)
	}

	return int64(info.NumPending), nil
}

// Replay republishes a DLQ message to its original topic.
func (r *DLQReader) Replay(ctx context.Context, seq uint64, publisher *Publisher) error {
	entry, err := r.Get(ctx, seq)
	if err != nil {
		return err
	}

	// OrgID and ProjectID are required for multi-tenant isolation
	if entry.Message.OrgID == "" {
		return fmt.Errorf("org_id is required for replay")
	}
	if entry.Message.ProjectID == "" {
		return fmt.Errorf("project_id is required for replay")
	}

	// Republish to original topic with org and project isolation
	event := struct {
		ID        string          `json:"id"`
		OrgID     string          `json:"org_id"`
		ProjectID string          `json:"project_id"`
		Topic     string          `json:"topic"`
		Data      json.RawMessage `json:"data"`
		Timestamp time.Time       `json:"timestamp"`
		Attempt   int             `json:"attempt"`
	}{
		ID:        entry.Message.ID,
		OrgID:     entry.Message.OrgID,
		ProjectID: entry.Message.ProjectID,
		Topic:     entry.Message.OriginalTopic,
		Data:      entry.Message.Data,
		Timestamp: entry.Message.Timestamp,
		Attempt:   1, // Reset attempt count
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Subject format: events.{org_id}.{project_id}.{topic}
	subject := "events." + entry.Message.OrgID + "." + entry.Message.ProjectID + "." + entry.Message.OriginalTopic
	_, err = r.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("republish event: %w", err)
	}

	// Delete from DLQ after successful replay
	return r.Delete(ctx, seq)
}

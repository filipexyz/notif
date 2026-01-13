package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// NATSAuditPublisher publishes audit events to NATS
type NATSAuditPublisher struct {
	js jetstream.JetStream
}

// NewNATSAuditPublisher creates a new NATS audit publisher
func NewNATSAuditPublisher(js jetstream.JetStream) *NATSAuditPublisher {
	return &NATSAuditPublisher{
		js: js,
	}
}

// PublishAudit publishes an audit event to NATS
func (p *NATSAuditPublisher) PublishAudit(orgID string, event AuditEvent) error {
	// Format as JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Create NATS subject: events.{org_id}.security.audit
	subject := fmt.Sprintf("events.%s.security.audit", orgID)

	// Publish to NATS (use context.Background() for fire-and-forget audit logging)
	ctx := context.Background()
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish audit event: %w", err)
	}

	return nil
}

// NullAuditPublisher is a no-op publisher (for when audit is disabled)
type NullAuditPublisher struct{}

// NewNullAuditPublisher creates a new null publisher
func NewNullAuditPublisher() *NullAuditPublisher {
	return &NullAuditPublisher{}
}

// PublishAudit does nothing
func (p *NullAuditPublisher) PublishAudit(orgID string, event AuditEvent) error {
	return nil
}

// BufferedAuditPublisher buffers audit events and publishes in batches
type BufferedAuditPublisher struct {
	publisher AuditPublisher
	buffer    chan AuditEvent
	batchSize int
}

// NewBufferedAuditPublisher creates a buffered publisher
func NewBufferedAuditPublisher(publisher AuditPublisher, bufferSize, batchSize int) *BufferedAuditPublisher {
	p := &BufferedAuditPublisher{
		publisher: publisher,
		buffer:    make(chan AuditEvent, bufferSize),
		batchSize: batchSize,
	}

	// Start background worker
	go p.worker()

	return p
}

// PublishAudit adds an audit event to the buffer
func (p *BufferedAuditPublisher) PublishAudit(orgID string, event AuditEvent) error {
	select {
	case p.buffer <- event:
		return nil
	default:
		// Buffer full - drop event (or we could block/return error)
		return fmt.Errorf("audit buffer full")
	}
}

// worker processes buffered events
func (p *BufferedAuditPublisher) worker() {
	batch := make([]AuditEvent, 0, p.batchSize)

	for event := range p.buffer {
		batch = append(batch, event)

		if len(batch) >= p.batchSize {
			p.flush(batch)
			batch = batch[:0]
		}
	}

	// Flush remaining events
	if len(batch) > 0 {
		p.flush(batch)
	}
}

func (p *BufferedAuditPublisher) flush(batch []AuditEvent) {
	for _, event := range batch {
		if err := p.publisher.PublishAudit(event.OrgID, event); err != nil {
			fmt.Printf("ERROR: Failed to publish audit event: %v\n", err)
		}
	}
}

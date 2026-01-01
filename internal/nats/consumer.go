package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// SubscriptionOptions configures a consumer subscription.
type SubscriptionOptions struct {
	Topics     []string
	OrgID      string // Required: filter by organization
	Group      string // Empty = ephemeral, non-empty = durable consumer group
	AutoAck    bool
	MaxRetries int
	AckTimeout time.Duration
	From       string // "latest" (default), "beginning", or timestamp
}

// DefaultSubscriptionOptions returns sensible defaults.
func DefaultSubscriptionOptions() SubscriptionOptions {
	return SubscriptionOptions{
		AutoAck:    false,
		MaxRetries: 5,
		AckTimeout: 5 * time.Minute,
	}
}

// ConsumerManager manages NATS consumers for subscriptions.
type ConsumerManager struct {
	stream jetstream.Stream
}

// NewConsumerManager creates a new ConsumerManager.
func NewConsumerManager(stream jetstream.Stream) *ConsumerManager {
	return &ConsumerManager{stream: stream}
}

// CreateConsumer creates a JetStream consumer for the given options.
func (cm *ConsumerManager) CreateConsumer(ctx context.Context, opts SubscriptionOptions) (jetstream.Consumer, error) {
	// OrgID is required for multi-tenant isolation
	if opts.OrgID == "" {
		return nil, fmt.Errorf("org_id is required for subscriptions")
	}

	// Convert topics to NATS subjects with org isolation
	// "leads.*" -> "events.{org_id}.leads.*"
	// "agent.*.error" -> "events.{org_id}.agent.*.error"
	// Special case: "*" -> "events.{org_id}.>" (match all topics)
	// In NATS, "*" matches exactly one token, while ">" matches one or more tokens.
	// A standalone "*" subscription should match all topics, including multi-segment
	// ones like "orders.created", so we convert it to ">" which is the NATS wildcard
	// for "one or more tokens".
	filterSubjects := make([]string, len(opts.Topics))
	for i, topic := range opts.Topics {
		if topic == "*" {
			// Standalone "*" means "all topics" - use ">" to match any depth
			filterSubjects[i] = "events." + opts.OrgID + ".>"
		} else {
			filterSubjects[i] = "events." + opts.OrgID + "." + topic
		}
	}

	// Determine deliver policy based on From option
	deliverPolicy := jetstream.DeliverNewPolicy // Default: only new messages
	var optStartTime time.Time
	switch opts.From {
	case "", "latest":
		deliverPolicy = jetstream.DeliverNewPolicy
	case "beginning":
		deliverPolicy = jetstream.DeliverAllPolicy
	default:
		// Try to parse as timestamp (RFC3339)
		if t, err := time.Parse(time.RFC3339, opts.From); err == nil {
			deliverPolicy = jetstream.DeliverByStartTimePolicy
			optStartTime = t
		}
		// If parsing fails, default to DeliverNewPolicy (latest)
	}

	config := jetstream.ConsumerConfig{
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        opts.AckTimeout,
		MaxDeliver:     opts.MaxRetries + 1,
		FilterSubjects: filterSubjects,
		DeliverPolicy:  deliverPolicy,
	}

	// Set OptStartTime if using DeliverByStartTimePolicy
	if deliverPolicy == jetstream.DeliverByStartTimePolicy {
		config.OptStartTime = &optStartTime
	}

	if opts.Group != "" {
		// Durable consumer for consumer groups (load balanced)
		config.Durable = opts.Group
		config.DeliverGroup = opts.Group
	}
	// Else: ephemeral consumer (unique per connection)

	consumer, err := cm.stream.CreateOrUpdateConsumer(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}

	return consumer, nil
}

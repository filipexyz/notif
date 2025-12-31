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
	Group      string // Empty = ephemeral, non-empty = durable consumer group
	AutoAck    bool
	MaxRetries int
	AckTimeout time.Duration
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
	// Convert topics to NATS subjects with wildcards
	// "leads.*" -> "events.leads.*"
	// "agent.*.error" -> "events.agent.*.error"
	filterSubjects := make([]string, len(opts.Topics))
	for i, topic := range opts.Topics {
		filterSubjects[i] = "events." + topic
	}

	config := jetstream.ConsumerConfig{
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        opts.AckTimeout,
		MaxDeliver:     opts.MaxRetries + 1,
		FilterSubjects: filterSubjects,
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

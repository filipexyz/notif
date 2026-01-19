package nats

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// SubscriptionOptions configures a consumer subscription.
type SubscriptionOptions struct {
	Topics     []string
	OrgID      string // Required: filter by organization
	ProjectID  string // Required: filter by project
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
	// OrgID and ProjectID are required for multi-tenant isolation
	if opts.OrgID == "" {
		return nil, fmt.Errorf("org_id is required for subscriptions")
	}
	if opts.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required for subscriptions")
	}

	// Convert topics to NATS subjects with org and project isolation
	// "leads.*" -> "events.{org_id}.{project_id}.leads.*"
	// "agent.*.error" -> "events.{org_id}.{project_id}.agent.*.error"
	// Special case: "*" -> "events.{org_id}.{project_id}.>" (match all topics)
	// In NATS, "*" matches exactly one token, while ">" matches one or more tokens.
	// A standalone "*" subscription should match all topics, including multi-segment
	// ones like "orders.created", so we convert it to ">" which is the NATS wildcard
	// for "one or more tokens".
	filterSubjects := make([]string, len(opts.Topics))
	for i, topic := range opts.Topics {
		if topic == "*" {
			// Standalone "*" means "all topics" - use ">" to match any depth
			filterSubjects[i] = "events." + opts.OrgID + "." + opts.ProjectID + ".>"
		} else {
			filterSubjects[i] = "events." + opts.OrgID + "." + opts.ProjectID + "." + topic
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
		// Include topic hash so different topic patterns get separate consumers
		consumerName := opts.Group + "-" + hashTopics(opts.Topics)
		config.Durable = consumerName
		config.DeliverGroup = consumerName
	}
	// Else: ephemeral consumer (unique per connection)

	consumer, err := cm.stream.CreateOrUpdateConsumer(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}

	return consumer, nil
}

// hashTopics returns a short hash of the sorted topics for consumer naming.
func hashTopics(topics []string) string {
	sorted := make([]string, len(topics))
	copy(sorted, topics)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return hex.EncodeToString(h[:])[:8]
}

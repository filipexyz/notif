package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	DefaultStreamName = "NOTIF_BRIDGE"
	DefaultMaxAge     = 24 * time.Hour
	MaxAckPending     = 1000
	HeartbeatInterval = 10 * time.Second
)

// Bridge connects to local NATS, creates a JetStream stream, and runs interceptors.
type Bridge struct {
	config     *Config
	configPath string

	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
	status *StatusReporter

	interceptors []runningInterceptor
	cancel       context.CancelFunc
}

// runningInterceptor holds a compiled interceptor and its consumer.
type runningInterceptor struct {
	config   InterceptorConfig
	jqCode   *gojq.Code
	consumer jetstream.Consumer
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewBridge creates a new Bridge instance.
func NewBridge(cfg *Config, configPath string) *Bridge {
	return &Bridge{
		config:     cfg,
		configPath: configPath,
	}
}

// Start connects to NATS, creates the stream, starts interceptors, and runs the heartbeat.
func (b *Bridge) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	// Initialize status reporter
	b.status = NewStatusReporter("", b.configPath, b.config)
	b.status.SetState(StateStarting)
	b.status.WriteHeartbeat()

	// Connect to NATS
	slog.Info("connecting to NATS", "url", b.config.NatsURL)
	nc, err := nats.Connect(b.config.NatsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("NATS disconnected", "error", err)
			b.status.SetNatsConnected(false)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("NATS reconnected")
			b.status.SetNatsConnected(true)
		}),
	)
	if err != nil {
		b.status.SetState(StateStopped)
		b.status.WriteHeartbeat()
		return fmt.Errorf("connect to NATS: %w", err)
	}
	b.nc = nc
	b.status.SetNatsConnected(true)
	slog.Info("connected to NATS", "url", b.config.NatsURL)

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return fmt.Errorf("create JetStream context: %w", err)
	}
	b.js = js

	// Create or reuse stream
	streamName := b.config.Stream
	if streamName == "" {
		streamName = DefaultStreamName
	}

	if len(b.config.Topics) > 0 {
		// Check for subject conflicts with existing streams
		if conflictStream, err := CheckStreamConflicts(ctx, js, b.config.Topics, streamName); err != nil {
			nc.Close()
			return fmt.Errorf("stream conflict with %q: %w", conflictStream, err)
		}

		stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
			Name:      streamName,
			Subjects:  b.config.Topics,
			Storage:   jetstream.FileStorage,
			Retention: jetstream.LimitsPolicy,
			MaxAge:    DefaultMaxAge,
			Replicas:  1,
			Discard:   jetstream.DiscardOld,
		})
		if err != nil {
			nc.Close()
			return fmt.Errorf("create stream %s: %w", streamName, err)
		}
		b.stream = stream
		b.status.SetStreamInfo(streamName, b.config.Topics)
		slog.Info("JetStream stream ready", "name", streamName, "subjects", b.config.Topics)
	}

	// Load and start interceptors
	if b.config.Interceptors != "" {
		if err := b.startInterceptors(ctx); err != nil {
			slog.Warn("failed to start interceptors", "error", err)
			// Non-fatal: bridge can run without interceptors
		}
	}

	b.status.SetState(StateRunning)
	b.status.WriteHeartbeat()

	// Start heartbeat loop
	go b.heartbeatLoop(ctx)

	slog.Info("bridge started")
	return nil
}

// Stop gracefully shuts down the bridge.
func (b *Bridge) Stop() {
	slog.Info("stopping bridge")

	if b.status != nil {
		b.status.SetState(StateStopping)
		b.status.WriteHeartbeat()
	}

	// Cancel interceptor contexts
	for _, ic := range b.interceptors {
		ic.cancel()
	}

	if b.cancel != nil {
		b.cancel()
	}

	if b.nc != nil {
		b.nc.Drain()
	}

	if b.status != nil {
		b.status.SetState(StateStopped)
		b.status.WriteHeartbeat()
		b.status.Cleanup()
	}

	slog.Info("bridge stopped")
}

// Status returns the current status data.
func (b *Bridge) Status() *StatusData {
	if b.status == nil {
		return &StatusData{State: StateStopped}
	}
	// Read from file to get the latest data
	s, err := ReadStatus("")
	if err != nil {
		return &StatusData{State: StateStopped}
	}
	return s
}

// CheckStreamConflicts checks if any existing stream captures the same subjects.
func CheckStreamConflicts(ctx context.Context, js jetstream.JetStream, topics []string, skipStream string) (string, error) {
	streamLister := js.ListStreams(ctx)
	for info := range streamLister.Info() {
		if info.Config.Name == skipStream {
			continue
		}
		if subjectsOverlap(info.Config.Subjects, topics) {
			return info.Config.Name, fmt.Errorf(
				"stream %q already captures subjects %v; use --stream %s to reuse it",
				info.Config.Name, info.Config.Subjects, info.Config.Name,
			)
		}
	}
	return "", nil
}

// subjectsOverlap checks if any subject in a overlaps with any in b using NATS matching rules.
func subjectsOverlap(a, b []string) bool {
	for _, sa := range a {
		for _, sb := range b {
			if natsSubjectOverlaps(sa, sb) {
				return true
			}
		}
	}
	return false
}

// natsSubjectOverlaps checks if two NATS subjects could match the same messages.
func natsSubjectOverlaps(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	return matchParts(aParts, 0, bParts, 0)
}

// matchParts recursively checks if two subject token sequences can overlap.
func matchParts(a []string, ai int, b []string, bi int) bool {
	// Both exhausted: match
	if ai >= len(a) && bi >= len(b) {
		return true
	}

	// One exhausted but other has ">" remaining: match
	if ai < len(a) && a[ai] == ">" {
		return bi < len(b) // ">" matches one or more tokens
	}
	if bi < len(b) && b[bi] == ">" {
		return ai < len(a)
	}

	// One exhausted, other not: no match
	if ai >= len(a) || bi >= len(b) {
		return false
	}

	// Either is "*": matches any single token
	if a[ai] == "*" || b[bi] == "*" {
		return matchParts(a, ai+1, b, bi+1)
	}

	// Literal match
	if a[ai] == b[bi] {
		return matchParts(a, ai+1, b, bi+1)
	}

	return false
}

// startInterceptors loads interceptor configs and starts consumers.
func (b *Bridge) startInterceptors(ctx context.Context) error {
	if b.stream == nil {
		return fmt.Errorf("no stream available for interceptors")
	}

	configs, err := LoadInterceptors(b.config.Interceptors)
	if err != nil {
		return fmt.Errorf("load interceptors: %w", err)
	}

	for _, icfg := range configs {
		// Compile jq expression
		query, err := gojq.Parse(icfg.JQ)
		if err != nil {
			slog.Error("invalid jq expression", "interceptor", icfg.Name, "error", err)
			continue
		}
		code, err := gojq.Compile(query)
		if err != nil {
			slog.Error("failed to compile jq", "interceptor", icfg.Name, "error", err)
			continue
		}

		// Create durable consumer
		consumerName := "interceptor-" + sanitizeName(icfg.Name)
		consumer, err := b.js.CreateOrUpdateConsumer(ctx, b.stream.CachedInfo().Config.Name, jetstream.ConsumerConfig{
			Durable:        consumerName,
			FilterSubjects: []string{icfg.From},
			AckPolicy:      jetstream.AckExplicitPolicy,
			MaxAckPending:  MaxAckPending,
		})
		if err != nil {
			slog.Error("failed to create consumer", "interceptor", icfg.Name, "error", err)
			continue
		}

		icCtx, icCancel := context.WithCancel(ctx)
		ri := runningInterceptor{
			config:   icfg,
			jqCode:   code,
			consumer: consumer,
			ctx:      icCtx,
			cancel:   icCancel,
		}
		b.interceptors = append(b.interceptors, ri)

		// Start consuming in background
		go b.runInterceptor(ri)

		slog.Info("interceptor started", "name", icfg.Name, "from", icfg.From, "to", icfg.To)
	}

	b.status.SetInterceptorCount(len(b.interceptors))
	return nil
}

// runInterceptor consumes messages and applies jq transforms.
func (b *Bridge) runInterceptor(ri runningInterceptor) {
	for {
		select {
		case <-ri.ctx.Done():
			return
		default:
		}

		msgs, err := ri.consumer.Fetch(100, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ri.ctx.Err() != nil {
				return // context cancelled
			}
			slog.Debug("fetch error", "interceptor", ri.config.Name, "error", err)
			continue
		}

		for msg := range msgs.Messages() {
			b.processMessage(ri, msg)
		}
		if err := msgs.Error(); err != nil {
			slog.Warn("fetch batch error", "interceptor", ri.config.Name, "error", err)
		}
	}
}

// processMessage applies a jq transform and publishes the result.
func (b *Bridge) processMessage(ri runningInterceptor, msg jetstream.Msg) {
	var input any
	if err := json.Unmarshal(msg.Data(), &input); err != nil {
		// Permanent failure: data won't change on retry
		slog.Error("unmarshal error, terminating message", "interceptor", ri.config.Name, "error", err)
		msg.Term()
		b.status.RecordError()
		return
	}

	iter := ri.jqCode.Run(input)
	v, ok := iter.Next()
	if !ok {
		msg.Ack()
		b.status.RecordProcessed(len(msg.Data()))
		return
	}
	if err, isErr := v.(error); isErr {
		// Permanent failure: same jq expression will always fail on same input
		slog.Error("jq error, terminating message", "interceptor", ri.config.Name, "error", err)
		msg.Term()
		b.status.RecordError()
		return
	}

	// Marshal result and publish to destination subject
	output, err := json.Marshal(v)
	if err != nil {
		// Permanent failure: same data will always fail to marshal
		slog.Error("marshal error, terminating message", "interceptor", ri.config.Name, "error", err)
		msg.Term()
		b.status.RecordError()
		return
	}

	// Build destination subject
	destSubject := ri.config.To
	if destSubject == "" {
		// No destination means transform in-place; just ack
		msg.Ack()
		b.status.RecordProcessed(len(msg.Data()))
		return
	}

	if err := b.nc.Publish(destSubject, output); err != nil {
		// Transient failure: NATS may recover
		slog.Error("publish error", "interceptor", ri.config.Name, "dest", destSubject, "error", err)
		msg.NakWithDelay(5 * time.Second)
		b.status.RecordError()
		return
	}

	msg.Ack()
	b.status.RecordProcessed(len(msg.Data()))
}

// heartbeatLoop writes status every HeartbeatInterval.
func (b *Bridge) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update NATS connection status
			if b.nc != nil {
				b.status.SetNatsConnected(b.nc.IsConnected())
			}
			if err := b.status.WriteHeartbeat(); err != nil {
				slog.Error("failed to write heartbeat", "error", err)
			}
		}
	}
}

// sanitizeName removes characters not suitable for NATS consumer names.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

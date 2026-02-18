package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/itchyny/gojq"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const headerKey = "X-Notif-Interceptor"

// Interceptor is a subscribe-transform-publish loop for reshaping NATS messages.
type Interceptor struct {
	name   string
	from   string
	to     string
	jq     *gojq.Code
	js     jetstream.JetStream
	stream jetstream.Stream
	logger *slog.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates an Interceptor. If jqExpr is empty, messages pass through unchanged.
func New(name, from, to, jqExpr string, js jetstream.JetStream, stream jetstream.Stream, logger *slog.Logger) (*Interceptor, error) {
	if name == "" {
		return nil, fmt.Errorf("interceptor name is required")
	}
	if strings.Contains(name, ",") {
		return nil, fmt.Errorf("interceptor %q: name must not contain commas", name)
	}
	if from == "" {
		return nil, fmt.Errorf("interceptor %q: from subject is required", name)
	}
	if to == "" {
		return nil, fmt.Errorf("interceptor %q: to subject is required", name)
	}
	var compiled *gojq.Code
	if jqExpr != "" {
		query, err := gojq.Parse(jqExpr)
		if err != nil {
			return nil, fmt.Errorf("parse jq expression: %w", err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("compile jq expression: %w", err)
		}
		compiled = code
	}
	return &Interceptor{
		name: name, from: from, to: to, jq: compiled,
		js: js, stream: stream, logger: logger,
	}, nil
}

// Start creates a durable consumer and begins processing messages.
func (i *Interceptor) Start(ctx context.Context) error {
	ctx, i.cancel = context.WithCancel(ctx)
	consumerName := "interceptor-" + i.name

	consumer, err := i.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:        consumerName,
		FilterSubjects: []string{i.from},
		AckPolicy:      jetstream.AckExplicitPolicy,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s: %w", consumerName, err)
	}

	cons, err := consumer.Consume(func(msg jetstream.Msg) { i.handleMessage(ctx, msg) })
	if err != nil {
		return fmt.Errorf("start consume %s: %w", consumerName, err)
	}

	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		<-ctx.Done()
		cons.Stop()
	}()

	i.logger.Info("interceptor started", "name", i.name, "from", i.from, "to", i.to)
	return nil
}

// Stop gracefully shuts down the interceptor.
func (i *Interceptor) Stop() {
	if i.cancel != nil {
		i.cancel()
	}
	i.wg.Wait()
	i.logger.Info("interceptor stopped", "name", i.name)
}

func (i *Interceptor) handleMessage(ctx context.Context, msg jetstream.Msg) {
	// Loop prevention: check if ANY interceptor in the chain is us
	var existingChain string
	if hdrs := msg.Headers(); hdrs != nil {
		existingChain = hdrs.Get(headerKey)
		if existingChain != "" {
			for _, name := range strings.Split(existingChain, ",") {
				if strings.TrimSpace(name) == i.name {
					_ = msg.Ack()
					return
				}
			}
		}
	}

	data := msg.Data()

	if i.jq != nil {
		var input interface{}
		if err := json.Unmarshal(data, &input); err != nil {
			i.logger.Error("unmarshal for jq", "error", err, "interceptor", i.name, "subject", msg.Subject())
			_ = msg.Ack()
			return
		}
		iter := i.jq.Run(input)
		v, ok := iter.Next()
		if !ok {
			_ = msg.Ack() // jq select filter dropped
			return
		}
		if err, isErr := v.(error); isErr {
			i.logger.Error("jq transform", "error", err, "interceptor", i.name, "subject", msg.Subject())
			_ = msg.Ack()
			return
		}
		var err error
		if data, err = json.Marshal(v); err != nil {
			i.logger.Error("marshal jq result", "error", err, "interceptor", i.name)
			_ = msg.Ack()
			return
		}
	}

	targetSubject := i.mapSubject(msg.Subject())
	outMsg := &nats.Msg{Subject: targetSubject, Data: data, Header: nats.Header{}}

	// Build interceptor chain: append our name to existing chain
	if existingChain != "" {
		outMsg.Header.Set(headerKey, existingChain+","+i.name)
	} else {
		outMsg.Header.Set(headerKey, i.name)
	}

	if _, err := i.js.PublishMsg(ctx, outMsg); err != nil {
		i.logger.Error("publish", "error", err, "interceptor", i.name, "subject", targetSubject)
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
	i.logger.Debug("interceptor processed", "name", i.name, "from", msg.Subject(), "to", targetSubject)
}

// mapSubject replaces the static prefix of `from` with the static prefix of `to`.
func (i *Interceptor) mapSubject(subject string) string {
	fromPrefix, toPrefix := staticPrefix(i.from), staticPrefix(i.to)
	if fromPrefix != "" && strings.HasPrefix(subject, fromPrefix) {
		return toPrefix + subject[len(fromPrefix):]
	}
	return toPrefix + subject
}

// staticPrefix returns the dot-joined prefix before the first wildcard token.
func staticPrefix(pattern string) string {
	parts := strings.Split(pattern, ".")
	var prefix []string
	for _, p := range parts {
		if p == "*" || p == ">" {
			break
		}
		prefix = append(prefix, p)
	}
	if len(prefix) == 0 {
		return ""
	}
	return strings.Join(prefix, ".") + "."
}

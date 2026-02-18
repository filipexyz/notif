package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"gopkg.in/yaml.v3"
)

type Config struct{ Bridges []BridgeConfig `yaml:"bridges"` }

type BridgeConfig struct {
	Name         string `yaml:"name"`
	URL          string `yaml:"url"`
	APIKey       string `yaml:"api_key"`
	Direction    string `yaml:"direction"`
	RemoteTopic  string `yaml:"remote_topic"`
	LocalSubject string `yaml:"local_subject"`
	Enabled      *bool  `yaml:"enabled"` // defaults to true if nil
}

// IsEnabled returns whether this bridge is enabled (defaults to true).
func (bc BridgeConfig) IsEnabled() bool {
	return bc.Enabled == nil || *bc.Enabled
}

// LoadConfig reads a YAML file and returns the parsed Config.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read federation config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse federation config: %w", err)
	}
	return &cfg, nil
}

type Bridge struct {
	name, direction, remoteTopic, localSubject, streamName string
	client                                                 *Client
	js                                                     jetstream.JetStream
	cancel                                                 context.CancelFunc
	wg                                                     sync.WaitGroup
}

type Federation struct {
	bridges []*Bridge
	logger  *slog.Logger
}

var envRe = regexp.MustCompile(`\$\{(\w+)\}`)

func expandEnv(s string) string {
	return envRe.ReplaceAllStringFunc(s, func(m string) string { return os.Getenv(envRe.FindStringSubmatch(m)[1]) })
}

func NewFederation(cfg *Config, js jetstream.JetStream, streamName string, logger *slog.Logger) (*Federation, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if streamName == "" {
		streamName = "NOTIF_EVENTS"
	}
	seen := make(map[string]bool)
	var bridges []*Bridge
	for _, bc := range cfg.Bridges {
		if !bc.IsEnabled() {
			logger.Info("federation bridge disabled, skipping", "name", bc.Name)
			continue
		}
		if bc.Name == "" {
			return nil, fmt.Errorf("bridge name is required")
		}
		if strings.Contains(bc.Name, ",") {
			return nil, fmt.Errorf("bridge %q: name must not contain commas", bc.Name)
		}
		if seen[bc.Name] {
			return nil, fmt.Errorf("duplicate bridge name: %q", bc.Name)
		}
		seen[bc.Name] = true
		if bc.URL == "" {
			return nil, fmt.Errorf("bridge %q: url is required", bc.Name)
		}
		if bc.Direction != "inbound" && bc.Direction != "outbound" {
			return nil, fmt.Errorf("bridge %q: invalid direction %q", bc.Name, bc.Direction)
		}
		if bc.RemoteTopic == "" {
			return nil, fmt.Errorf("bridge %q: remote_topic is required", bc.Name)
		}
		if bc.LocalSubject == "" {
			return nil, fmt.Errorf("bridge %q: local_subject is required", bc.Name)
		}
		bridges = append(bridges, &Bridge{
			name: bc.Name, direction: bc.Direction,
			remoteTopic: bc.RemoteTopic, localSubject: bc.LocalSubject, streamName: streamName,
			client: NewClient(bc.URL, expandEnv(bc.APIKey), logger), js: js,
		})
	}
	return &Federation{bridges: bridges, logger: logger}, nil
}

func (f *Federation) Start(ctx context.Context) error {
	for i, b := range f.bridges {
		bCtx, cancel := context.WithCancel(ctx)
		b.cancel = cancel

		var err error
		if b.direction == "inbound" {
			err = b.startInbound(bCtx, f.logger)
		} else {
			err = b.startOutbound(bCtx, f.logger)
		}
		if err != nil {
			cancel()
			// Rollback previously started bridges
			for j := 0; j < i; j++ {
				if f.bridges[j].cancel != nil {
					f.bridges[j].cancel()
				}
				f.bridges[j].wg.Wait()
			}
			return fmt.Errorf("bridge %q: %w", b.name, err)
		}
		f.logger.Info("federation bridge started", "name", b.name, "direction", b.direction)
	}
	return nil
}

func (f *Federation) Stop() {
	for _, b := range f.bridges {
		if b.cancel != nil {
			b.cancel()
		}
		b.wg.Wait()
	}
}

func (b *Bridge) startInbound(ctx context.Context, logger *slog.Logger) error {
	events, err := b.client.Subscribe(ctx, []string{b.remoteTopic})
	if err != nil {
		return err
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for evt := range events {
			payload, err := json.Marshal(map[string]any{"id": evt.ID, "topic": evt.Topic, "data": evt.Data, "timestamp": evt.Timestamp})
			if err != nil {
				logger.Error("federation: marshal inbound event failed", "bridge", b.name, "error", err)
				continue
			}
			if _, err := b.js.Publish(ctx, b.localSubject, payload); err != nil {
				logger.Error("federation: local publish failed", "bridge", b.name, "error", err, "subject", b.localSubject)
			}
		}
	}()
	return nil
}

func (b *Bridge) startOutbound(ctx context.Context, logger *slog.Logger) error {
	consumer, err := b.js.CreateOrUpdateConsumer(ctx, b.streamName, jetstream.ConsumerConfig{
		Durable:       "federation-" + b.name,
		FilterSubject: b.localSubject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("create outbound consumer on stream %s: %w", b.streamName, err)
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		cc, err := consumer.Consume(func(msg jetstream.Msg) {
			var evt struct{ Data json.RawMessage `json:"data"` }
			if json.Unmarshal(msg.Data(), &evt) != nil || evt.Data == nil {
				evt.Data = msg.Data()
			}
			if err := b.client.Emit(ctx, b.remoteTopic, evt.Data); err != nil {
				logger.Error("federation: remote emit failed", "bridge", b.name, "error", err)
				msg.Nak()
				return
			}
			msg.Ack()
		})
		if err != nil {
			logger.Error("federation: outbound consume", "bridge", b.name, "error", err)
			return
		}
		<-ctx.Done()
		cc.Stop()
	}()
	return nil
}

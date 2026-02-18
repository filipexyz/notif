package interceptor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

// Manager creates and manages a set of interceptors from config.
type Manager struct {
	interceptors []*Interceptor
	logger       *slog.Logger
}

// NewManager builds interceptors from config. Only enabled entries are created.
func NewManager(cfg *Config, js jetstream.JetStream, stream jetstream.Stream, logger *slog.Logger) (*Manager, error) {
	seen := make(map[string]bool)
	var interceptors []*Interceptor
	for _, ic := range cfg.Interceptors {
		if !ic.IsEnabled() {
			logger.Info("interceptor disabled, skipping", "name", ic.Name)
			continue
		}
		if seen[ic.Name] {
			return nil, fmt.Errorf("duplicate interceptor name: %q", ic.Name)
		}
		seen[ic.Name] = true
		intc, err := New(ic.Name, ic.From, ic.To, ic.Jq, js, stream, logger)
		if err != nil {
			return nil, fmt.Errorf("create interceptor %s: %w", ic.Name, err)
		}
		interceptors = append(interceptors, intc)
	}
	return &Manager{interceptors: interceptors, logger: logger}, nil
}

// Start starts all interceptors. Rolls back previously started ones on failure.
func (m *Manager) Start(ctx context.Context) error {
	for i, intc := range m.interceptors {
		if err := intc.Start(ctx); err != nil {
			// Rollback previously started interceptors
			for j := 0; j < i; j++ {
				m.interceptors[j].Stop()
			}
			return fmt.Errorf("start interceptor %s: %w", intc.name, err)
		}
	}
	m.logger.Info("interceptor manager started", "count", len(m.interceptors))
	return nil
}

// Stop stops all interceptors.
func (m *Manager) Stop() {
	for _, intc := range m.interceptors {
		intc.Stop()
	}
	m.logger.Info("interceptor manager stopped")
}

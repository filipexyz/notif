package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kardianos/service"
)

const (
	ServiceName        = "notif-connect"
	ServiceDisplayName = "Notif Connect Bridge"
	ServiceDescription = "Bridges local NATS to notif.sh cloud via interceptors and leaf node protocol"
)

// ServiceProgram implements kardianos/service.Interface.
type ServiceProgram struct {
	bridge     *Bridge
	configPath string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewServiceProgram creates a new ServiceProgram.
func NewServiceProgram(configPath string) *ServiceProgram {
	return &ServiceProgram{
		configPath: configPath,
	}
}

// Start is called by the service manager to start the service.
func (p *ServiceProgram) Start(s service.Service) error {
	slog.Info("service starting")

	cfg, err := LoadConfig(p.configPath)
	if err != nil {
		return err
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.bridge = NewBridge(cfg, p.configPath)

	// Start bridge in background but wait for startup result
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.bridge.Start(p.ctx)
	}()

	select {
	case err = <-errCh:
		if err != nil {
			return fmt.Errorf("bridge start failed: %w", err)
		}
		return nil
	case <-time.After(30 * time.Second):
		p.cancel()
		return fmt.Errorf("bridge start timed out after 30s")
	}
}

// Stop is called by the service manager to stop the service.
func (p *ServiceProgram) Stop(s service.Service) error {
	slog.Info("service stopping")
	if p.bridge != nil {
		p.bridge.Stop()
	}
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// NewServiceConfig returns the kardianos service configuration.
func NewServiceConfig() *service.Config {
	return &service.Config{
		Name:        ServiceName,
		DisplayName: ServiceDisplayName,
		Description: ServiceDescription,
		Arguments:   []string{"connect", "run", "--config", DefaultConfigPath()},
	}
}

// GetService creates a kardianos service instance.
func GetService(configPath string) (service.Service, error) {
	prg := NewServiceProgram(configPath)
	svcConfig := NewServiceConfig()
	if configPath != "" {
		svcConfig.Arguments = []string{"connect", "run", "--config", configPath}
	}
	return service.New(prg, svcConfig)
}

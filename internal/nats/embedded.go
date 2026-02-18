package nats

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

// EmbeddedConfig configures the in-process NATS server.
type EmbeddedConfig struct {
	StoreDir string // base dir for JetStream + JWT resolver data
	Host     string // bind address (default "127.0.0.1")
	Port     int    // client port (default 4222, -1 for random)

	// Multi-account auth (leave empty for plain JetStream, no auth)
	OperatorPublicKey      string
	SystemAccountPublicKey string
}

// EmbeddedServer wraps an in-process NATS server.
type EmbeddedServer struct {
	server *natsserver.Server
}

// StartEmbedded starts a NATS server in-process.
func StartEmbedded(cfg EmbeddedConfig) (*EmbeddedServer, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 4222
	}

	opts := &natsserver.Options{
		Host:       cfg.Host,
		Port:       cfg.Port,
		JetStream:  true,
		StoreDir:   filepath.Join(cfg.StoreDir, "jetstream"),
		MaxPayload: 1 << 20, // 1MB
		NoSigs:     true,
		NoLog:      true,
	}

	// Multi-account: operator trust + system account + dir resolver
	if cfg.OperatorPublicKey != "" && cfg.SystemAccountPublicKey != "" {
		operatorClaims := jwt.NewOperatorClaims(cfg.OperatorPublicKey)
		operatorClaims.SystemAccount = cfg.SystemAccountPublicKey

		opts.TrustedOperators = []*jwt.OperatorClaims{operatorClaims}
		opts.SystemAccount = cfg.SystemAccountPublicKey

		jwtDir := filepath.Join(cfg.StoreDir, "jwt")
		resolver, err := natsserver.NewDirAccResolver(jwtDir, 0, time.Minute, natsserver.NoDelete)
		if err != nil {
			return nil, fmt.Errorf("create dir resolver: %w", err)
		}
		opts.AccountResolver = resolver
	}

	srv, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create embedded NATS server: %w", err)
	}

	srv.Start()

	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		return nil, fmt.Errorf("embedded NATS server failed to become ready")
	}

	slog.Info("embedded NATS server started", "url", srv.ClientURL())

	return &EmbeddedServer{server: srv}, nil
}

// ClientURL returns the URL clients should connect to.
func (e *EmbeddedServer) ClientURL() string {
	return e.server.ClientURL()
}

// Shutdown gracefully stops the embedded server.
func (e *EmbeddedServer) Shutdown() {
	if e.server != nil {
		e.server.Shutdown()
		slog.Info("embedded NATS server stopped")
	}
}

package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/policy"
	"github.com/filipexyz/notif/internal/webhook"
	"github.com/filipexyz/notif/internal/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is the HTTP server.
type Server struct {
	cfg            *config.Config
	db             *pgxpool.Pool
	nats           *nats.Client
	hub            *websocket.Hub
	server         *http.Server
	webhookCancel  context.CancelFunc
	policyEnforcer *policy.Enforcer
	policyLoader   *policy.Loader
}

// New creates a new Server.
func New(cfg *config.Config, pool *pgxpool.Pool, nc *nats.Client) *Server {
	// Initialize Clerk for dashboard authentication
	if cfg.ClerkSecretKey != "" {
		clerk.SetKey(cfg.ClerkSecretKey)
		slog.Info("Clerk authentication enabled for dashboard routes")
	} else {
		slog.Warn("CLERK_SECRET_KEY not set - dashboard routes will not work")
	}

	hub := websocket.NewHub()
	go hub.Run()

	// Initialize policy system
	policyDir := os.Getenv("NOTIF_POLICY_DIR")
	if policyDir == "" {
		policyDir = "/etc/notif/policies"
	}

	var policyEnforcer *policy.Enforcer
	var policyLoader *policy.Loader

	loader, err := policy.NewLoader(policyDir)
	if err != nil {
		slog.Warn("Failed to initialize policy loader, policies disabled", "error", err)
	} else {
		slog.Info("Policy system initialized", "policy_dir", policyDir)
		policyLoader = loader

		// Setup audit publisher
		auditPublisher := policy.NewNATSAuditPublisher(nc.JetStream())
		auditor := policy.NewAuditor(auditPublisher)

		// Create enforcer
		policyEnforcer = policy.NewEnforcer(loader, auditor)
	}

	s := &Server{
		cfg:            cfg,
		db:             pool,
		nats:           nc,
		hub:            hub,
		policyEnforcer: policyEnforcer,
		policyLoader:   policyLoader,
	}

	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.routes(),
	}

	// ============================================================================
	// WARNING: Webhook worker runs in-process for simplicity.
	//
	// This approach is fine for low-to-medium traffic, but has limitations:
	// - No horizontal scaling of webhook delivery independent of the API server
	// - Slow webhook endpoints can consume goroutines/connections
	// - Server restart interrupts in-flight deliveries
	//
	// For high-volume production use, consider running the webhook worker as a
	// separate process that can be scaled independently.
	// ============================================================================
	webhookCtx, webhookCancel := context.WithCancel(context.Background())
	s.webhookCancel = webhookCancel

	queries := db.New(s.db)
	dlqPublisher := nats.NewDLQPublisher(nc.JetStream())
	worker := webhook.NewWorker(queries, nc.Stream(), nc.JetStream(), dlqPublisher)
	go func() {
		if err := worker.Start(webhookCtx); err != nil && webhookCtx.Err() == nil {
			slog.Error("webhook worker error", "error", err)
		}
	}()

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Serve starts the HTTP server on the given listener.
func (s *Server) Serve(l net.Listener) error {
	return s.server.Serve(l)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop webhook worker first
	if s.webhookCancel != nil {
		s.webhookCancel()
	}

	// Stop policy loader
	if s.policyLoader != nil {
		if err := s.policyLoader.Close(); err != nil {
			slog.Error("failed to close policy loader", "error", err)
		}
	}

	return s.server.Shutdown(ctx)
}

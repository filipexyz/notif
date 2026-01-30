package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/scheduler"
	"github.com/filipexyz/notif/internal/terminal"
	"github.com/filipexyz/notif/internal/webhook"
	"github.com/filipexyz/notif/internal/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is the HTTP server.
type Server struct {
	cfg              *config.Config
	db               *pgxpool.Pool
	nats             *nats.Client
	hub              *websocket.Hub
	terminalManager  *terminal.Manager
	schedulerWorker  *scheduler.Worker
	rateLimiter      *middleware.RateLimiter
	server           *http.Server
	webhookCancel    context.CancelFunc
	schedulerCancel  context.CancelFunc
}

// New creates a new Server.
func New(cfg *config.Config, pool *pgxpool.Pool, nc *nats.Client) *Server {
	// Log auth mode
	if cfg.IsSelfHosted() {
		slog.Info("Running in self-hosted mode",
			"auth_mode", string(cfg.AuthMode),
			"default_org", cfg.DefaultOrgID,
		)
		slog.Info("Bootstrap your instance: POST /api/v1/bootstrap")
	} else {
		// Initialize Clerk for dashboard authentication
		if cfg.ClerkSecretKey != "" {
			clerk.SetKey(cfg.ClerkSecretKey)
			slog.Info("Clerk authentication enabled for dashboard routes")
		} else {
			slog.Warn("CLERK_SECRET_KEY not set - dashboard routes will not work")
		}
	}

	hub := websocket.NewHub()
	go hub.Run()

	// Terminal manager for web terminal sessions
	serverURL := "http://localhost:" + cfg.Port
	termMgr := terminal.NewManager(cfg.CLIBinaryPath, serverURL)

	// Initialize scheduler worker
	queries := db.New(pool)
	publisher := nats.NewPublisher(nc.JetStream())
	schedWorker := scheduler.NewWorker(queries, publisher, 10*time.Second)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())

	s := &Server{
		cfg:              cfg,
		db:               pool,
		nats:             nc,
		hub:              hub,
		terminalManager:  termMgr,
		schedulerWorker:  schedWorker,
		rateLimiter:      rateLimiter,
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

	dlqPublisher := nats.NewDLQPublisher(nc.JetStream())
	worker := webhook.NewWorker(queries, nc.Stream(), nc.JetStream(), dlqPublisher)
	go func() {
		if err := worker.Start(webhookCtx); err != nil && webhookCtx.Err() == nil {
			slog.Error("webhook worker error", "error", err)
		}
	}()

	// Start scheduler worker
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	s.schedulerCancel = schedulerCancel
	go schedWorker.Start(schedulerCtx)

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
	// Stop workers first
	if s.webhookCancel != nil {
		s.webhookCancel()
	}
	if s.schedulerCancel != nil {
		s.schedulerCancel()
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	return s.server.Shutdown(ctx)
}

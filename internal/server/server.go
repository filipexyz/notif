package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/filipexyz/notif/internal/accounts"
	"github.com/filipexyz/notif/internal/audit"
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
	cfg             *config.Config
	db              *pgxpool.Pool
	nats            *nats.Client      // legacy single-connection mode
	pool            *nats.ClientPool  // multi-account mode
	accountMgr      *accounts.Manager // multi-account mode
	hub             *websocket.Hub
	terminalManager *terminal.Manager
	schedulerWorker *scheduler.Worker
	rateLimiter     *middleware.RateLimiter
	auditLog        *audit.Logger
	server          *http.Server
	webhookCtx      context.Context    // lifetime context for webhook workers
	webhookCancel   context.CancelFunc
	orgWorkerMu     sync.Mutex                    // guards orgWorkerCancels
	orgWorkerCancels map[string]context.CancelFunc // per-org webhook worker cancellation
	schedulerCancel context.CancelFunc
}

// New creates a new Server in legacy single-connection mode.
func New(cfg *config.Config, pool *pgxpool.Pool, nc *nats.Client) *Server {
	initClerk(cfg)

	hub := websocket.NewHub()
	go hub.Run()

	serverURL := "http://localhost:" + cfg.Port
	termMgr := terminal.NewManager(cfg.CLIBinaryPath, serverURL)

	queries := db.New(pool)
	publisher := nats.NewPublisher(nc.JetStream())
	schedWorker := scheduler.NewWorker(queries, publisher, 10*time.Second)

	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())
	auditLog := audit.New(queries, 256)

	s := &Server{
		cfg:             cfg,
		db:              pool,
		nats:            nc,
		hub:             hub,
		terminalManager: termMgr,
		schedulerWorker: schedWorker,
		rateLimiter:     rateLimiter,
		auditLog:        auditLog,
	}

	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.routes(),
	}

	// Start webhook worker
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

// NewWithPool creates a new Server in multi-account mode using ClientPool.
func NewWithPool(cfg *config.Config, dbPool *pgxpool.Pool, pool *nats.ClientPool, accountMgr *accounts.Manager, auditLog *audit.Logger) *Server {
	initClerk(cfg)

	hub := websocket.NewHub()
	go hub.Run()

	serverURL := "http://localhost:" + cfg.Port
	termMgr := terminal.NewManager(cfg.CLIBinaryPath, serverURL)

	queries := db.New(dbPool)
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultRateLimitConfig())

	s := &Server{
		cfg:             cfg,
		db:              dbPool,
		pool:            pool,
		accountMgr:      accountMgr,
		hub:             hub,
		terminalManager: termMgr,
		rateLimiter:     rateLimiter,
		auditLog:        auditLog,
	}

	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.routes(),
	}

	// Start webhook workers for each org
	// NOTE: Scheduler is disabled in multi-account mode until per-org scheduling is implemented.
	webhookCtx, webhookCancel := context.WithCancel(context.Background())
	s.webhookCtx = webhookCtx
	s.webhookCancel = webhookCancel
	s.orgWorkerCancels = make(map[string]context.CancelFunc)

	for _, orgID := range pool.OrgIDs() {
		s.startOrgWorker(orgID, queries)
	}

	return s
}

// startOrgWorker is the shared implementation for starting a per-org webhook worker.
func (s *Server) startOrgWorker(orgID string, queries *db.Queries) {
	orgClient, err := s.pool.Get(orgID)
	if err != nil {
		slog.Error("failed to get org client for webhook worker", "org_id", orgID, "error", err)
		return
	}

	orgCtx, orgCancel := context.WithCancel(s.webhookCtx)

	s.orgWorkerMu.Lock()
	s.orgWorkerCancels[orgID] = orgCancel
	s.orgWorkerMu.Unlock()

	dlqPublisher := nats.NewDLQPublisher(orgClient.JetStream())
	worker := webhook.NewWorker(queries, orgClient.Stream(), orgClient.JetStream(), dlqPublisher)
	go func(oid string) {
		if err := worker.Start(orgCtx); err != nil && orgCtx.Err() == nil {
			slog.Error("webhook worker error", "org_id", oid, "error", err)
		}
	}(orgID)

	slog.Info("webhook worker started", "org_id", orgID)
}

// StartOrgWebhookWorker starts a webhook delivery worker for a dynamically-created org.
// Called by OrgHandler.Create after a new org is added to the pool.
func (s *Server) StartOrgWebhookWorker(orgID string) {
	if s.pool == nil || s.webhookCtx == nil {
		return
	}
	s.startOrgWorker(orgID, db.New(s.db))
}

// StopOrgWebhookWorker stops the webhook worker for a deleted org.
// Called by OrgHandler.Delete before the org is removed from the pool.
func (s *Server) StopOrgWebhookWorker(orgID string) {
	s.orgWorkerMu.Lock()
	cancel, ok := s.orgWorkerCancels[orgID]
	if ok {
		delete(s.orgWorkerCancels, orgID)
	}
	s.orgWorkerMu.Unlock()

	if ok {
		cancel()
		slog.Info("webhook worker stopped", "org_id", orgID)
	}
}

func initClerk(cfg *config.Config) {
	if cfg.IsSelfHosted() {
		slog.Info("Running in self-hosted mode",
			"auth_mode", string(cfg.AuthMode),
			"default_org", cfg.DefaultOrgID,
		)
		slog.Info("Bootstrap your instance: POST /api/v1/bootstrap")
	} else {
		if cfg.ClerkSecretKey != "" {
			clerk.SetKey(cfg.ClerkSecretKey)
			slog.Info("Clerk authentication enabled for dashboard routes")
		} else {
			slog.Warn("CLERK_SECRET_KEY not set - dashboard routes will not work")
		}
	}
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
	if s.webhookCancel != nil {
		s.webhookCancel()
	}
	if s.schedulerCancel != nil {
		s.schedulerCancel()
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	// Shutdown HTTP server first (drains inflight requests),
	// then close audit logger (safe: no more Log() calls after server stops).
	err := s.server.Shutdown(ctx)
	if s.auditLog != nil {
		s.auditLog.Close()
	}
	return err
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/federation"
	"github.com/filipexyz/notif/internal/interceptor"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logging
	setupLogging(cfg)

	// Connect to Postgres
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Connect to NATS
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("connected to NATS")

	// Ensure JetStream streams exist
	if err := nc.EnsureStreams(ctx); err != nil {
		slog.Error("failed to setup JetStream streams", "error", err)
		os.Exit(1)
	}

	// Start interceptors (optional — hard fail if config path is set but invalid)
	var interceptorMgr *interceptor.Manager
	if cfg.InterceptorsConfigPath != "" {
		icfg, err := interceptor.LoadConfig(cfg.InterceptorsConfigPath)
		if err != nil {
			slog.Error("failed to load interceptors config", "error", err)
			os.Exit(1)
		}
		interceptorMgr, err = interceptor.NewManager(icfg, nc.JetStream(), nc.Stream(), slog.Default())
		if err != nil {
			slog.Error("failed to create interceptor manager", "error", err)
			os.Exit(1)
		}
		if err := interceptorMgr.Start(ctx); err != nil {
			slog.Error("failed to start interceptors", "error", err)
			os.Exit(1)
		}
		slog.Info("interceptors started", "config", cfg.InterceptorsConfigPath)
	}

	// Start federation (optional — hard fail if config path is set but invalid)
	var fed *federation.Federation
	if cfg.FederationConfigPath != "" {
		fcfg, err := federation.LoadConfig(cfg.FederationConfigPath)
		if err != nil {
			slog.Error("failed to load federation config", "error", err)
			os.Exit(1)
		}
		fed, err = federation.NewFederation(fcfg, nc.JetStream(), nats.StreamName, slog.Default())
		if err != nil {
			slog.Error("failed to create federation", "error", err)
			os.Exit(1)
		}
		if err := fed.Start(ctx); err != nil {
			slog.Error("failed to start federation", "error", err)
			os.Exit(1)
		}
		slog.Info("federation started", "config", cfg.FederationConfigPath)
	}

	// Create and start HTTP server
	srv := server.New(cfg, db, nc)

	go func() {
		slog.Info("starting server", "port", cfg.Port)
		if err := srv.Start(); err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down...")

	// Graceful shutdown: HTTP first, then interceptors/federation, then NATS
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	// Stop interceptors and federation after HTTP is drained (they may still
	// have in-flight messages to publish) but before NATS connection closes.
	if interceptorMgr != nil {
		interceptorMgr.Stop()
	}
	if fed != nil {
		fed.Stop()
	}

	slog.Info("shutdown complete")
}

func setupLogging(cfg *config.Config) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{}
	switch cfg.LogLevel {
	case "debug":
		opts.Level = slog.LevelDebug
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		opts.Level = slog.LevelInfo
	}

	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

package server

import (
	"context"
	"net"
	"net/http"

	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is the HTTP server.
type Server struct {
	cfg    *config.Config
	db     *pgxpool.Pool
	nats   *nats.Client
	hub    *websocket.Hub
	server *http.Server
}

// New creates a new Server.
func New(cfg *config.Config, db *pgxpool.Pool, nc *nats.Client) *Server {
	hub := websocket.NewHub()
	go hub.Run()

	s := &Server{
		cfg:  cfg,
		db:   db,
		nats: nc,
		hub:  hub,
	}

	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.routes(),
	}

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
	return s.server.Shutdown(ctx)
}

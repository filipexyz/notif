package server

import (
	"net/http"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/handler"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)

	// Health checks (no auth)
	healthHandler := handler.NewHealthHandler(s.db, s.nats)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// API routes (require auth)
	queries := db.New(s.db)
	authMiddleware := middleware.NewAuth(queries)
	publisher := nats.NewPublisher(s.nats.JetStream())
	emitHandler := handler.NewEmitHandler(publisher, queries)

	consumerMgr := nats.NewConsumerManager(s.nats.Stream())
	subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)

		r.Post("/emit", emitHandler.Emit)
		r.Get("/subscribe", subscribeHandler.Subscribe)
	})

	return r
}

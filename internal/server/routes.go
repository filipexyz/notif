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

	queries := db.New(s.db)

	// ================================================================
	// API v1 ROUTES
	// Unified auth: accepts both API key (Bearer nsh_xxx) and Clerk JWT
	// ================================================================
	publisher := nats.NewPublisher(s.nats.JetStream())
	emitHandler := handler.NewEmitHandler(publisher, queries)

	consumerMgr := nats.NewConsumerManager(s.nats.Stream())
	dlqPublisher := nats.NewDLQPublisher(s.nats.JetStream())
	subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr, dlqPublisher)

	dlqReader, _ := nats.NewDLQReader(s.nats.JetStream())
	dlqHandler := handler.NewDLQHandler(dlqReader, publisher)

	eventReader := nats.NewEventReader(s.nats.Stream())
	eventsHandler := handler.NewEventsHandler(eventReader)

	webhookHandler := handler.NewWebhookHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)

	// WebSocket endpoint at root (no /api/v1 prefix for WS)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries))
		r.Get("/ws", subscribeHandler.Subscribe)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries))

		// Events
		r.Post("/emit", emitHandler.Emit)
		r.Get("/events", eventsHandler.List)
		r.Get("/events/stats", eventsHandler.Stats)
		r.Get("/events/{seq}", eventsHandler.Get)

		// Webhooks
		r.Post("/webhooks", webhookHandler.Create)
		r.Get("/webhooks", webhookHandler.List)
		r.Get("/webhooks/{id}", webhookHandler.Get)
		r.Put("/webhooks/{id}", webhookHandler.Update)
		r.Delete("/webhooks/{id}", webhookHandler.Delete)
		r.Get("/webhooks/{id}/deliveries", webhookHandler.Deliveries)

		// DLQ
		r.Get("/dlq", dlqHandler.List)
		r.Get("/dlq/{seq}", dlqHandler.Get)
		r.Post("/dlq/{seq}/replay", dlqHandler.Replay)
		r.Delete("/dlq/{seq}", dlqHandler.Delete)
		r.Post("/dlq/replay-all", dlqHandler.ReplayAll)
		r.Delete("/dlq/purge", dlqHandler.Purge)

		// Stats (observability)
		r.Get("/stats/overview", statsHandler.Overview)
		r.Get("/stats/events", statsHandler.Events)
		r.Get("/stats/webhooks", statsHandler.Webhooks)
		r.Get("/stats/dlq", statsHandler.DLQ)

		// API Keys (requires Clerk auth, not API key)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireClerkAuth())

			r.Post("/api-keys", apiKeyHandler.Create)
			r.Get("/api-keys", apiKeyHandler.List)
			r.Delete("/api-keys/{id}", apiKeyHandler.Revoke)
		})
	})

	return r
}

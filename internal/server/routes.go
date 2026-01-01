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
	// DASHBOARD ROUTES (Clerk JWT Authentication)
	// Used by web dashboard for account management
	// ================================================================
	clerkAuth := middleware.NewClerkAuth()
	apiKeyHandler := handler.NewAPIKeyHandler(queries)

	r.Route("/dashboard", func(r chi.Router) {
		r.Use(clerkAuth.Handler)

		// API Key Management
		r.Post("/api-keys", apiKeyHandler.Create)
		r.Get("/api-keys", apiKeyHandler.List)
		r.Delete("/api-keys/{id}", apiKeyHandler.Revoke)
	})

	// ================================================================
	// API ROUTES (API Key Authentication)
	// Used by CLI, SDKs, and programmatic access
	// ================================================================
	authMiddleware := middleware.NewAuth(queries)
	publisher := nats.NewPublisher(s.nats.JetStream())
	emitHandler := handler.NewEmitHandler(publisher, queries)

	consumerMgr := nats.NewConsumerManager(s.nats.Stream())
	dlqPublisher := nats.NewDLQPublisher(s.nats.JetStream())
	subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr, dlqPublisher)

	// DLQ handler
	dlqReader, _ := nats.NewDLQReader(s.nats.JetStream())
	dlqHandler := handler.NewDLQHandler(dlqReader, publisher)

	// Events handler
	eventReader := nats.NewEventReader(s.nats.Stream())
	eventsHandler := handler.NewEventsHandler(eventReader)

	// Webhook handler
	webhookHandler := handler.NewWebhookHandler(queries)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Handler)

		r.Post("/emit", emitHandler.Emit)
		r.Get("/subscribe", subscribeHandler.Subscribe)

		// Events query endpoints
		r.Get("/events", eventsHandler.List)
		r.Get("/events/stats", eventsHandler.Stats)
		r.Get("/events/{seq}", eventsHandler.Get)

		// Webhook endpoints
		r.Post("/webhooks", webhookHandler.Create)
		r.Get("/webhooks", webhookHandler.List)
		r.Get("/webhooks/{id}", webhookHandler.Get)
		r.Put("/webhooks/{id}", webhookHandler.Update)
		r.Delete("/webhooks/{id}", webhookHandler.Delete)
		r.Get("/webhooks/{id}/deliveries", webhookHandler.Deliveries)

		// DLQ endpoints
		r.Get("/dlq", dlqHandler.List)
		r.Get("/dlq/{seq}", dlqHandler.Get)
		r.Post("/dlq/{seq}/replay", dlqHandler.Replay)
		r.Delete("/dlq/{seq}", dlqHandler.Delete)
		r.Post("/dlq/replay-all", dlqHandler.ReplayAll)
		r.Delete("/dlq/purge", dlqHandler.Purge)
	})

	return r
}

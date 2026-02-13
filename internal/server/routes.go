package server

import (
	"net/http"

	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/handler"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)

	// Clerk JWT parsing (non-blocking - just parses if present)
	// Also handles query param 'token' for WebSocket connections
	r.Use(middleware.ClerkQueryParamAuth())
	r.Use(clerkhttp.WithHeaderAuthorization())

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Project-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	queries := db.New(s.db)

	// Health checks (no auth)
	healthHandler := handler.NewHealthHandler(s.db, s.nats)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Bootstrap endpoints for self-hosted setup (no auth required)
	bootstrapHandler := handler.NewBootstrapHandler(queries, s.cfg)
	r.Get("/api/v1/bootstrap/status", bootstrapHandler.Status)
	r.Post("/api/v1/bootstrap", bootstrapHandler.Bootstrap)

	// ================================================================
	// API v1 ROUTES
	// Unified auth: accepts both API key (Bearer nsh_xxx) and Clerk JWT
	// ================================================================
	publisher := nats.NewPublisher(s.nats.JetStream())
	schemaRegistry := schema.NewRegistry(queries)
	emitHandler := handler.NewEmitHandler(publisher, queries, schemaRegistry, s.cfg)

	consumerMgr := nats.NewConsumerManager(s.nats.Stream())
	dlqPublisher := nats.NewDLQPublisher(s.nats.JetStream())
	subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr, dlqPublisher, queries, s.cfg)

	dlqReader, _ := nats.NewDLQReader(s.nats.JetStream())
	dlqHandler := handler.NewDLQHandler(dlqReader, publisher)

	eventReader := nats.NewEventReader(s.nats.Stream())
	eventsHandler := handler.NewEventsHandler(eventReader, queries)

	webhookHandler := handler.NewWebhookHandler(queries)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
	schedulesHandler := handler.NewSchedulesHandler(queries, s.schedulerWorker)
	projectHandler := handler.NewProjectHandler(queries)

	schemaHandler := handler.NewSchemaHandler(schemaRegistry)

	// WebSocket endpoint at root (no /api/v1 prefix for WS)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries, s.cfg))
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Get("/ws", subscribeHandler.Subscribe)
	})

	// Terminal WebSocket endpoint (requires Clerk JWT, not API key)
	terminalHandler := handler.NewTerminalHandler(s.terminalManager, s.cfg.CORSOrigins)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries, s.cfg))
		r.Use(middleware.RequireClerkAuth(s.cfg))
		r.Get("/ws/terminal", terminalHandler.HandleWS)
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Rate limit BEFORE auth to protect against brute force
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Use(middleware.UnifiedAuth(queries, s.cfg))

		// Events
		r.Post("/emit", emitHandler.Emit)
		r.Get("/events", eventsHandler.List)
		r.Get("/events/stats", eventsHandler.Stats)
		r.Get("/events/{seq}", eventsHandler.Get)
		r.Get("/events/{id}/deliveries", eventsHandler.Deliveries)

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

		// Schedules
		r.Post("/schedules", schedulesHandler.Create)
		r.Get("/schedules", schedulesHandler.List)
		r.Get("/schedules/{id}", schedulesHandler.Get)
		r.Delete("/schedules/{id}", schedulesHandler.Cancel)
		r.Post("/schedules/{id}/run", schedulesHandler.Run)

		// Schemas
		r.Post("/schemas", schemaHandler.CreateSchema)
		r.Get("/schemas", schemaHandler.ListSchemas)
		r.Get("/schemas/for-topic/{topic}", schemaHandler.GetSchemaForTopic)
		r.Get("/schemas/{name}", schemaHandler.GetSchema)
		r.Put("/schemas/{name}", schemaHandler.UpdateSchema)
		r.Delete("/schemas/{name}", schemaHandler.DeleteSchema)
		r.Post("/schemas/{name}/versions", schemaHandler.CreateVersion)
		r.Get("/schemas/{name}/versions", schemaHandler.ListVersions)
		r.Get("/schemas/{name}/versions/{version}", schemaHandler.GetVersion)
		r.Post("/schemas/{name}/validate", schemaHandler.Validate)

		// Stats (observability)
		r.Get("/stats/overview", statsHandler.Overview)
		r.Get("/stats/events", statsHandler.Events)
		r.Get("/stats/webhooks", statsHandler.Webhooks)
		r.Get("/stats/dlq", statsHandler.DLQ)
		r.Get("/stats/schedules", schedulesHandler.Stats)

		// Dashboard routes (requires Clerk auth, not API key)
		// In self-hosted mode, allows API key auth instead
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireClerkAuth(s.cfg))

			// API Keys
			r.Post("/api-keys", apiKeyHandler.Create)
			r.Get("/api-keys", apiKeyHandler.List)
			r.Delete("/api-keys/{id}", apiKeyHandler.Revoke)

			// Projects
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{id}", projectHandler.Get)
			r.Put("/projects/{id}", projectHandler.Update)
			r.Delete("/projects/{id}", projectHandler.Delete)
		})
	})

	return r
}

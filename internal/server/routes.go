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
	if s.pool != nil {
		// Multi-account mode: use pool-aware health handler
		healthHandler := handler.NewPoolHealthHandler(s.db, s.pool)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
		r.Get("/healthz", healthHandler.Healthz)
	} else {
		healthHandler := handler.NewHealthHandler(s.db, s.nats)
		r.Get("/health", healthHandler.Health)
		r.Get("/ready", healthHandler.Ready)
	}

	// Bootstrap endpoints for self-hosted setup (no auth, but rate limited)
	bootstrapHandler := handler.NewBootstrapHandler(queries, s.cfg)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Get("/api/v1/bootstrap/status", bootstrapHandler.Status)
		r.Post("/api/v1/bootstrap", bootstrapHandler.Bootstrap)
	})

	// Build handlers based on mode
	if s.pool != nil {
		s.routesMultiAccount(r, queries)
	} else {
		s.routesLegacy(r, queries)
	}

	return r
}

// routesMultiAccount sets up routes for multi-account mode using ClientPool.
func (s *Server) routesMultiAccount(r chi.Router, queries *db.Queries) {
	schemaRegistry := schema.NewRegistry(queries)

	// Org management endpoints (admin only)
	orgHandler := handler.NewOrgHandler(queries, s.pool, s.accountMgr, s.auditLog)
	orgHandler.SetOnOrgCreated(s.StartOrgWebhookWorker)
	orgHandler.SetOnOrgDeleted(s.StopOrgWebhookWorker)
	r.Route("/api/v1/orgs", func(r chi.Router) {
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Use(middleware.UnifiedAuth(queries, s.cfg))
		r.Use(middleware.RequireClerkAuth(s.cfg))

		r.Post("/", orgHandler.Create)
		r.Get("/", orgHandler.List)
		r.Delete("/{id}", orgHandler.Delete)
		r.Get("/{id}/limits", orgHandler.Limits)
		r.Put("/{id}/limits", orgHandler.Limits)
	})

	// WebSocket endpoint
	r.Group(func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries, s.cfg))
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				http.Error(w, "org not found in pool", http.StatusServiceUnavailable)
				return
			}

			consumerMgr := nats.NewConsumerManager(orgClient.Stream())
			dlqPublisher := nats.NewDLQPublisher(orgClient.JetStream())
			subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr, dlqPublisher, queries, s.cfg, s.auditLog)
			subscribeHandler.Subscribe(w, r)
		})
	})

	// Terminal WebSocket endpoint
	terminalHandler := handler.NewTerminalHandler(s.terminalManager, s.cfg.CORSOrigins)
	r.Group(func(r chi.Router) {
		r.Use(middleware.UnifiedAuth(queries, s.cfg))
		r.Use(middleware.RequireClerkAuth(s.cfg))
		r.Get("/ws/terminal", terminalHandler.HandleWS)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Use(middleware.UnifiedAuth(queries, s.cfg))

		// Events — resolve orgID → pool.Get(orgID)
		r.Post("/emit", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{
					"error": "org not connected",
				})
				return
			}

			publisher := nats.NewPublisher(orgClient.JetStream())
			emitHandler := handler.NewEmitHandler(publisher, queries, schemaRegistry, s.cfg, s.auditLog)
			emitHandler.Emit(w, r)
		})

		r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}

			eventReader := nats.NewEventReader(orgClient.Stream())
			eventsHandler := handler.NewEventsHandler(eventReader, queries)
			eventsHandler.List(w, r)
		})
		r.Get("/events/stats", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			eventsHandler := handler.NewEventsHandler(eventReader, queries)
			eventsHandler.Stats(w, r)
		})
		r.Get("/events/{seq}", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			eventsHandler := handler.NewEventsHandler(eventReader, queries)
			eventsHandler.Get(w, r)
		})
		r.Get("/events/{id}/deliveries", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			eventsHandler := handler.NewEventsHandler(eventReader, queries)
			eventsHandler.Deliveries(w, r)
		})

		// Webhooks
		webhookHandler := handler.NewWebhookHandler(queries, s.auditLog)
		r.Post("/webhooks", webhookHandler.Create)
		r.Get("/webhooks", webhookHandler.List)
		r.Get("/webhooks/{id}", webhookHandler.Get)
		r.Put("/webhooks/{id}", webhookHandler.Update)
		r.Delete("/webhooks/{id}", webhookHandler.Delete)
		r.Get("/webhooks/{id}/deliveries", webhookHandler.Deliveries)

		// DLQ — resolve orgID → pool.Get(orgID) for per-account DLQ
		r.Get("/dlq", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.List(w, r)
		})
		r.Get("/dlq/{seq}", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.Get(w, r)
		})
		r.Post("/dlq/{seq}/replay", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.Replay(w, r)
		})
		r.Delete("/dlq/{seq}", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.Delete(w, r)
		})
		r.Post("/dlq/replay-all", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.ReplayAll(w, r)
		})
		r.Delete("/dlq/purge", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			publisher := nats.NewPublisher(orgClient.JetStream())
			dlqHandler := handler.NewDLQHandler(dlqReader, publisher)
			dlqHandler.Purge(w, r)
		})

		// Schedules — disabled in multi-account mode until per-org scheduling is implemented.
		// Each org needs its own scheduler worker; the current single-worker design would
		// route all schedules to a single org's JetStream.
		notImplemented := func(w http.ResponseWriter, r *http.Request) {
			handler.WriteJSONPublic(w, http.StatusNotImplemented, map[string]string{
				"error": "schedules not yet available in multi-account mode",
			})
		}
		r.Post("/schedules", http.HandlerFunc(notImplemented))
		r.Get("/schedules", http.HandlerFunc(notImplemented))
		r.Get("/schedules/{id}", http.HandlerFunc(notImplemented))
		r.Delete("/schedules/{id}", http.HandlerFunc(notImplemented))
		r.Post("/schedules/{id}/run", http.HandlerFunc(notImplemented))
		r.Get("/stats/schedules", http.HandlerFunc(notImplemented))

		// Schemas
		schemaRegistry := schema.NewRegistry(queries)
		schemaHandler := handler.NewSchemaHandler(schemaRegistry)
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

		// Audit log
		auditHandler := handler.NewAuditHandler(queries)
		r.Get("/audit", auditHandler.List)

		// Stats — resolve per org
		r.Get("/stats/overview", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
			statsHandler.Overview(w, r)
		})
		r.Get("/stats/events", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
			statsHandler.Events(w, r)
		})
		r.Get("/stats/webhooks", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
			statsHandler.Webhooks(w, r)
		})
		r.Get("/stats/dlq", func(w http.ResponseWriter, r *http.Request) {
			authCtx := middleware.GetAuthContext(r.Context())
			if authCtx == nil || authCtx.OrgID == "" {
				handler.WriteJSONPublic(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			orgClient, err := s.pool.Get(authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "org not connected"})
				return
			}
			eventReader := nats.NewEventReader(orgClient.Stream())
			dlqReader, err := nats.NewDLQReaderForOrg(orgClient.JetStream(), authCtx.OrgID)
			if err != nil {
				handler.WriteJSONPublic(w, http.StatusServiceUnavailable, map[string]string{"error": "DLQ not available"})
				return
			}
			statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
			statsHandler.DLQ(w, r)
		})

		// Dashboard routes (requires Clerk auth)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireClerkAuth(s.cfg))

			apiKeyHandler := handler.NewAPIKeyHandler(queries)
			r.Post("/api-keys", apiKeyHandler.Create)
			r.Get("/api-keys", apiKeyHandler.List)
			r.Delete("/api-keys/{id}", apiKeyHandler.Revoke)

			projectHandler := handler.NewProjectHandler(queries)
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{id}", projectHandler.Get)
			r.Put("/projects/{id}", projectHandler.Update)
			r.Delete("/projects/{id}", projectHandler.Delete)
		})
	})
}

// routesLegacy sets up routes for legacy single-connection mode (unchanged behavior).
func (s *Server) routesLegacy(r chi.Router, queries *db.Queries) {
	publisher := nats.NewPublisher(s.nats.JetStream())
	schemaRegistry := schema.NewRegistry(queries)
	emitHandler := handler.NewEmitHandler(publisher, queries, schemaRegistry, s.cfg, s.auditLog)

	consumerMgr := nats.NewConsumerManager(s.nats.Stream())
	dlqPublisher := nats.NewDLQPublisher(s.nats.JetStream())
	subscribeHandler := handler.NewSubscribeHandler(s.hub, consumerMgr, dlqPublisher, queries, s.cfg, s.auditLog)

	dlqReader, _ := nats.NewDLQReader(s.nats.JetStream())
	dlqHandler := handler.NewDLQHandler(dlqReader, publisher)

	eventReader := nats.NewEventReader(s.nats.Stream())
	eventsHandler := handler.NewEventsHandler(eventReader, queries)

	webhookHandler := handler.NewWebhookHandler(queries, s.auditLog)
	apiKeyHandler := handler.NewAPIKeyHandler(queries)
	statsHandler := handler.NewStatsHandler(queries, eventReader, dlqReader)
	schedulesHandler := handler.NewSchedulesHandler(queries, s.schedulerWorker)
	projectHandler := handler.NewProjectHandler(queries)

	schemaHandler := handler.NewSchemaHandler(schemaRegistry)
	auditHandler := handler.NewAuditHandler(queries)

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
		r.Use(middleware.RateLimit(s.rateLimiter))
		r.Use(middleware.UnifiedAuth(queries, s.cfg))

		r.Post("/emit", emitHandler.Emit)
		r.Get("/events", eventsHandler.List)
		r.Get("/events/stats", eventsHandler.Stats)
		r.Get("/events/{seq}", eventsHandler.Get)
		r.Get("/events/{id}/deliveries", eventsHandler.Deliveries)

		r.Post("/webhooks", webhookHandler.Create)
		r.Get("/webhooks", webhookHandler.List)
		r.Get("/webhooks/{id}", webhookHandler.Get)
		r.Put("/webhooks/{id}", webhookHandler.Update)
		r.Delete("/webhooks/{id}", webhookHandler.Delete)
		r.Get("/webhooks/{id}/deliveries", webhookHandler.Deliveries)

		r.Get("/dlq", dlqHandler.List)
		r.Get("/dlq/{seq}", dlqHandler.Get)
		r.Post("/dlq/{seq}/replay", dlqHandler.Replay)
		r.Delete("/dlq/{seq}", dlqHandler.Delete)
		r.Post("/dlq/replay-all", dlqHandler.ReplayAll)
		r.Delete("/dlq/purge", dlqHandler.Purge)

		r.Post("/schedules", schedulesHandler.Create)
		r.Get("/schedules", schedulesHandler.List)
		r.Get("/schedules/{id}", schedulesHandler.Get)
		r.Delete("/schedules/{id}", schedulesHandler.Cancel)
		r.Post("/schedules/{id}/run", schedulesHandler.Run)

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

		r.Get("/audit", auditHandler.List)

		r.Get("/stats/overview", statsHandler.Overview)
		r.Get("/stats/events", statsHandler.Events)
		r.Get("/stats/webhooks", statsHandler.Webhooks)
		r.Get("/stats/dlq", statsHandler.DLQ)
		r.Get("/stats/schedules", schedulesHandler.Stats)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireClerkAuth(s.cfg))

			r.Post("/api-keys", apiKeyHandler.Create)
			r.Get("/api-keys", apiKeyHandler.List)
			r.Delete("/api-keys/{id}", apiKeyHandler.Revoke)

			r.Post("/projects", projectHandler.Create)
			r.Get("/projects", projectHandler.List)
			r.Get("/projects/{id}", projectHandler.Get)
			r.Put("/projects/{id}", projectHandler.Update)
			r.Delete("/projects/{id}", projectHandler.Delete)
		})
	})
}

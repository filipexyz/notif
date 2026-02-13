package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/jackc/pgx/v5/pgtype"
)

// EmitHandler handles POST /emit.
type EmitHandler struct {
	publisher      *nats.Publisher
	queries        *db.Queries
	schemaRegistry *schema.Registry
	cfg            *config.Config
}

// NewEmitHandler creates a new EmitHandler.
func NewEmitHandler(publisher *nats.Publisher, queries *db.Queries, schemaRegistry *schema.Registry, cfg *config.Config) *EmitHandler {
	return &EmitHandler{
		publisher:      publisher,
		queries:        queries,
		schemaRegistry: schemaRegistry,
		cfg:            cfg,
	}
}

// Emit publishes an event to a topic.
func (h *EmitHandler) Emit(w http.ResponseWriter, r *http.Request) {
	// Limit body size
	maxSize := h.cfg.MaxPayloadSize
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	var req domain.EmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": fmt.Sprintf("payload too large, max %dKB", maxSize/1024),
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON: " + err.Error(),
		})
		return
	}

	// Validate topic
	if err := validateTopic(req.Topic); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Schema validation (if registry is configured and we have project context)
	authCtx := middleware.GetAuthContext(r.Context())
	if h.schemaRegistry != nil && authCtx != nil && authCtx.ProjectID != "" {
		validationResult, err := h.schemaRegistry.ValidateEvent(r.Context(), authCtx.ProjectID, req.Topic, req.Data)
		if err != nil {
			slog.Error("schema validation error", "error", err, "topic", req.Topic)
			// Don't block on validation errors - treat as no schema
		} else if validationResult != nil && !validationResult.Valid {
			// Get the schema to check validation mode
			schemaForTopic, _ := h.schemaRegistry.GetSchemaForTopic(r.Context(), authCtx.ProjectID, req.Topic)
			if schemaForTopic != nil && schemaForTopic.LatestVersion != nil {
				switch schemaForTopic.LatestVersion.ValidationMode {
				case schema.ValidationModeStrict:
					switch schemaForTopic.LatestVersion.OnInvalid {
					case schema.OnInvalidReject:
						writeJSON(w, http.StatusBadRequest, map[string]any{
							"error":             "schema validation failed",
							"schema":            validationResult.Schema,
							"version":           validationResult.Version,
							"validation_errors": validationResult.Errors,
						})
						return
					case schema.OnInvalidLog, schema.OnInvalidDLQ:
						// Log but continue
						slog.Warn("schema validation failed",
							"topic", req.Topic,
							"schema", validationResult.Schema,
							"errors", validationResult.Errors,
						)
					}
				case schema.ValidationModeWarn:
					slog.Warn("schema validation warning",
						"topic", req.Topic,
						"schema", validationResult.Schema,
						"errors", validationResult.Errors,
					)
				// ValidationModeDisabled - do nothing
				}
			}
		}
	}

	// Create event with org and project context
	event := domain.NewEvent(req.Topic, req.Data)
	if authCtx != nil {
		event.OrgID = authCtx.OrgID
		event.ProjectID = authCtx.ProjectID
	}

	// Publish to NATS
	if err := h.publisher.Publish(r.Context(), event); err != nil {
		slog.Error("failed to publish event", "error", err, "topic", req.Topic)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to publish event",
		})
		return
	}

	// Store event metadata (sync, ensures event exists for delivery queries)
	apiKey := middleware.GetAPIKey(r.Context())
	if authCtx != nil && authCtx.OrgID != "" {
		params := db.CreateEventParams{
			ID:          event.ID,
			Topic:       event.Topic,
			OrgID:       authCtx.OrgID,
			ProjectID:   pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
			PayloadSize: int32(len(req.Data)),
			CreatedAt:   pgtype.Timestamptz{Time: event.Timestamp, Valid: true},
		}
		if apiKey != nil {
			params.ApiKeyID = apiKey.ID
		}
		if err := h.queries.CreateEvent(r.Context(), params); err != nil {
			slog.Error("failed to store event metadata", "error", err, "event_id", event.ID)
			// Don't fail the request, event was already published to NATS
		}
	}

	slog.Info("event emitted",
		"event_id", event.ID,
		"topic", event.Topic,
		"size", len(req.Data),
	)

	writeJSON(w, http.StatusOK, domain.EmitResponse{
		ID:        event.ID,
		Topic:     event.Topic,
		CreatedAt: event.Timestamp,
	})
}

func validateTopic(topic string) error {
	if topic == "" {
		return &validationError{"topic is required"}
	}
	if len(topic) > 255 {
		return &validationError{"topic too long, max 255 chars"}
	}
	if strings.HasPrefix(topic, "$") {
		return &validationError{"topic cannot start with $"}
	}
	if strings.HasPrefix(topic, ".") || strings.HasSuffix(topic, ".") {
		return &validationError{"topic cannot start or end with ."}
	}
	if strings.ContainsAny(topic, ">*") {
		return &validationError{"topic cannot contain wildcard characters (> or *)"}
	}
	return nil
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

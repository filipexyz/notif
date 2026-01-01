package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxPayloadSize = 64 * 1024 // 64KB

// EmitHandler handles POST /emit.
type EmitHandler struct {
	publisher *nats.Publisher
	queries   *db.Queries
}

// NewEmitHandler creates a new EmitHandler.
func NewEmitHandler(publisher *nats.Publisher, queries *db.Queries) *EmitHandler {
	return &EmitHandler{
		publisher: publisher,
		queries:   queries,
	}
}

// Emit publishes an event to a topic.
func (h *EmitHandler) Emit(w http.ResponseWriter, r *http.Request) {
	// Limit body size
	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)

	var req domain.EmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": "payload too large, max 64KB",
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

	// Create event
	event := domain.NewEvent(req.Topic, req.Data)

	// Publish to NATS
	if err := h.publisher.Publish(r.Context(), event); err != nil {
		slog.Error("failed to publish event", "error", err, "topic", req.Topic)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to publish event",
		})
		return
	}

	// Store event metadata (async, don't block response)
	apiKey := middleware.GetAPIKey(r.Context())
	if apiKey != nil {
		go func() {
			_ = h.queries.CreateEvent(r.Context(), db.CreateEventParams{
				ID:          event.ID,
				Topic:       event.Topic,
				ApiKeyID:    apiKey.ID,
				PayloadSize: int32(len(req.Data)),
				CreatedAt:   pgtype.Timestamptz{Time: event.Timestamp, Valid: true},
			})
		}()
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

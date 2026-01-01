package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// EventsHandler handles event query operations.
type EventsHandler struct {
	reader  *nats.EventReader
	queries *db.Queries
}

// NewEventsHandler creates a new EventsHandler.
func NewEventsHandler(reader *nats.EventReader, queries *db.Queries) *EventsHandler {
	return &EventsHandler{reader: reader, queries: queries}
}

// List returns historical events filtered by org.
func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	opts := nats.QueryOptions{
		Topic: r.URL.Query().Get("topic"),
		OrgID: authCtx.OrgID,
		Limit: 100,
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			opts.Limit = l
			if opts.Limit > 1000 {
				opts.Limit = 1000
			}
		}
	}

	// Parse from timestamp
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			opts.From = t
		} else if ts, err := strconv.ParseInt(fromStr, 10, 64); err == nil {
			opts.From = time.Unix(ts, 0)
		}
	}

	// Parse to timestamp
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			opts.To = t
		} else if ts, err := strconv.ParseInt(toStr, 10, 64); err == nil {
			opts.To = time.Unix(ts, 0)
		}
	}

	events, err := h.reader.Query(r.Context(), opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to query events: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"count":  len(events),
	})
}

// Get returns a specific event by sequence number (with org verification).
func (h *EventsHandler) Get(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	seqStr := chi.URLParam(r, "seq")
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid sequence number",
		})
		return
	}

	event, err := h.reader.GetBySeq(r.Context(), seq)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "event not found",
		})
		return
	}

	// Verify org ownership - critical security check
	if event.Event.OrgID != authCtx.OrgID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "event not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, event)
}

// Stats returns stream statistics.
func (h *EventsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	info, err := h.reader.StreamInfo(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to get stream info: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages":    info.State.Msgs,
		"bytes":       info.State.Bytes,
		"first_seq":   info.State.FirstSeq,
		"last_seq":    info.State.LastSeq,
		"first_time":  info.State.FirstTime,
		"last_time":   info.State.LastTime,
		"consumers":   info.State.Consumers,
	})
}

// Deliveries returns all deliveries (webhooks and websocket) for a specific event.
func (h *EventsHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	eventID := chi.URLParam(r, "id")
	if eventID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event id required"})
		return
	}

	// Get unified deliveries with webhook URLs (org-scoped)
	deliveries, err := h.queries.GetEventDeliveriesWithWebhookURL(r.Context(), db.GetEventDeliveriesWithWebhookURLParams{
		EventID: eventID,
		OrgID:   authCtx.OrgID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get deliveries"})
		return
	}

	// Format for JSON response
	results := make([]map[string]any, len(deliveries))
	for i, d := range deliveries {
		results[i] = map[string]any{
			"id":            uuid.UUID(d.ID.Bytes).String(),
			"event_id":      d.EventID,
			"receiver_type": d.ReceiverType,
			"status":        d.Status,
			"attempt":       d.Attempt,
			"created_at":    d.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}

		// Add receiver-specific fields
		if d.ReceiverType == "webhook" {
			if d.ReceiverID.Valid {
				results[i]["receiver_id"] = uuid.UUID(d.ReceiverID.Bytes).String()
			}
			if d.WebhookUrl.Valid {
				results[i]["webhook_url"] = d.WebhookUrl.String
			}
		} else {
			if d.ConsumerName.Valid {
				results[i]["consumer_name"] = d.ConsumerName.String
			}
			if d.ClientID.Valid {
				results[i]["client_id"] = d.ClientID.String
			}
		}

		if d.DeliveredAt.Valid {
			results[i]["delivered_at"] = d.DeliveredAt.Time.Format("2006-01-02T15:04:05Z")
		}
		if d.AckedAt.Valid {
			results[i]["acked_at"] = d.AckedAt.Time.Format("2006-01-02T15:04:05Z")
		}
		if d.Error.Valid {
			results[i]["error"] = d.Error.String
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries": results,
		"count":      len(results),
	})
}

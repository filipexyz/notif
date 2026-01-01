package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/filipexyz/notif/internal/nats"
	"github.com/go-chi/chi/v5"
)

// EventsHandler handles event query operations.
type EventsHandler struct {
	reader *nats.EventReader
}

// NewEventsHandler creates a new EventsHandler.
func NewEventsHandler(reader *nats.EventReader) *EventsHandler {
	return &EventsHandler{reader: reader}
}

// List returns historical events.
func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	opts := nats.QueryOptions{
		Topic: r.URL.Query().Get("topic"),
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

// Get returns a specific event by sequence number.
func (h *EventsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

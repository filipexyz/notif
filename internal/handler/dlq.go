package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/go-chi/chi/v5"
)

// DLQHandler handles DLQ operations.
type DLQHandler struct {
	reader    *nats.DLQReader
	publisher *nats.Publisher
}

// NewDLQHandler creates a new DLQHandler.
func NewDLQHandler(reader *nats.DLQReader, publisher *nats.Publisher) *DLQHandler {
	return &DLQHandler{
		reader:    reader,
		publisher: publisher,
	}
}

// List returns messages from the DLQ (project-scoped).
func (h *DLQHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	topic := r.URL.Query().Get("topic")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries, err := h.reader.List(r.Context(), authCtx.OrgID, authCtx.ProjectID, topic, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list DLQ",
		})
		return
	}

	// Flatten response for frontend
	messages := make([]map[string]any, len(entries))
	for i, entry := range entries {
		messages[i] = map[string]any{
			"seq":        entry.Seq,
			"topic":      entry.Message.OriginalTopic,
			"error":      entry.Message.LastError,
			"attempts":   entry.Message.Attempts,
			"created_at": entry.Message.FailedAt,
			"event_id":   entry.Message.ID,
			"data":       entry.Message.Data,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"count":    len(messages),
	})
}

// Get returns a specific DLQ message (with org and project verification).
func (h *DLQHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	entry, err := h.reader.Get(r.Context(), seq)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}

	// Verify org and project ownership
	if entry.Message.OrgID != authCtx.OrgID || entry.Message.ProjectID != authCtx.ProjectID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// Replay republishes a DLQ message to its original topic (with org and project verification).
func (h *DLQHandler) Replay(w http.ResponseWriter, r *http.Request) {
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

	// Verify org and project ownership before replay
	entry, err := h.reader.Get(r.Context(), seq)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}
	if entry.Message.OrgID != authCtx.OrgID || entry.Message.ProjectID != authCtx.ProjectID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}

	if err := h.reader.Replay(r.Context(), seq, h.publisher); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to replay message",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "replayed",
	})
}

// Delete removes a message from the DLQ (with org and project verification).
func (h *DLQHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	// Verify org and project ownership before delete
	entry, err := h.reader.Get(r.Context(), seq)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}
	if entry.Message.OrgID != authCtx.OrgID || entry.Message.ProjectID != authCtx.ProjectID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "message not found",
		})
		return
	}

	if err := h.reader.Delete(r.Context(), seq); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete message",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

// ReplayAll replays all messages from the DLQ (project-scoped), optionally filtered by topic.
func (h *DLQHandler) ReplayAll(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	topic := r.URL.Query().Get("topic")

	entries, err := h.reader.List(r.Context(), authCtx.OrgID, authCtx.ProjectID, topic, 1000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list DLQ",
		})
		return
	}

	replayed := 0
	failed := 0
	for _, entry := range entries {
		if err := h.reader.Replay(r.Context(), entry.Seq, h.publisher); err != nil {
			failed++
		} else {
			replayed++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"replayed": replayed,
		"failed":   failed,
	})
}

// Purge deletes all messages from the DLQ (project-scoped), optionally filtered by topic.
func (h *DLQHandler) Purge(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	topic := r.URL.Query().Get("topic")

	entries, err := h.reader.List(r.Context(), authCtx.OrgID, authCtx.ProjectID, topic, 1000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list DLQ",
		})
		return
	}

	deleted := 0
	for _, entry := range entries {
		if err := h.reader.Delete(r.Context(), entry.Seq); err == nil {
			deleted++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
	})
}

func writeJSONDLQ(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

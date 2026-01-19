package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/scheduler"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SchedulesHandler handles scheduled events endpoints.
type SchedulesHandler struct {
	queries   *db.Queries
	scheduler *scheduler.Worker
}

// NewSchedulesHandler creates a new SchedulesHandler.
func NewSchedulesHandler(queries *db.Queries, scheduler *scheduler.Worker) *SchedulesHandler {
	return &SchedulesHandler{
		queries:   queries,
		scheduler: scheduler,
	}
}

// CreateScheduleRequest is the request body for POST /schedules.
type CreateScheduleRequest struct {
	Topic        string          `json:"topic"`
	Data         json.RawMessage `json:"data"`
	ScheduledFor *time.Time      `json:"scheduled_for,omitempty"`
	In           string          `json:"in,omitempty"`
}

// CreateScheduleResponse is the response body for POST /schedules.
type CreateScheduleResponse struct {
	ID           string    `json:"id"`
	Topic        string    `json:"topic"`
	ScheduledFor time.Time `json:"scheduled_for"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScheduleResponse is the response body for GET /schedules/:id.
type ScheduleResponse struct {
	ID           string          `json:"id"`
	Topic        string          `json:"topic"`
	Data         json.RawMessage `json:"data"`
	ScheduledFor time.Time       `json:"scheduled_for"`
	Status       string          `json:"status"`
	Error        *string         `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	ExecutedAt   *time.Time      `json:"executed_at,omitempty"`
}

// RunScheduleResponse is the response body for POST /schedules/:id/run.
type RunScheduleResponse struct {
	ScheduleID string `json:"schedule_id"`
	EventID    string `json:"event_id"`
}

// Create handles POST /schedules.
func (h *SchedulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Validate topic
	if err := validateTopic(req.Topic); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Calculate scheduled_for
	var scheduledFor time.Time
	if req.ScheduledFor != nil {
		scheduledFor = *req.ScheduledFor
	} else if req.In != "" {
		duration, err := time.ParseDuration(req.In)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid duration: " + err.Error()})
			return
		}
		scheduledFor = time.Now().Add(duration)
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scheduled_for or in is required"})
		return
	}

	// Validate scheduled_for is in the future
	if scheduledFor.Before(time.Now()) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scheduled_for must be in the future"})
		return
	}

	// Generate schedule ID
	id := generateScheduleID()

	// Get API key for audit
	var apiKeyID pgtype.UUID
	if apiKey := middleware.GetAPIKey(r.Context()); apiKey != nil {
		apiKeyID = apiKey.ID
	}

	// Create scheduled event
	sch, err := h.queries.CreateScheduledEvent(r.Context(), db.CreateScheduledEventParams{
		ID:           id,
		OrgID:        authCtx.OrgID,
		ProjectID:    pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
		Topic:        req.Topic,
		Data:         req.Data,
		ScheduledFor: pgtype.Timestamptz{Time: scheduledFor, Valid: true},
		ApiKeyID:     apiKeyID,
	})
	if err != nil {
		slog.Error("failed to create scheduled event", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create scheduled event"})
		return
	}

	slog.Info("scheduled event created",
		"id", sch.ID,
		"topic", sch.Topic,
		"scheduled_for", scheduledFor,
	)

	writeJSON(w, http.StatusCreated, CreateScheduleResponse{
		ID:           sch.ID,
		Topic:        sch.Topic,
		ScheduledFor: sch.ScheduledFor.Time,
		CreatedAt:    sch.CreatedAt.Time,
	})
}

// List handles GET /schedules.
func (h *SchedulesHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse query params
	limit := 50
	offset := 0
	status := r.URL.Query().Get("status")

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var schedules []db.ScheduledEvent
	var err error

	if status != "" {
		schedules, err = h.queries.ListScheduledEventsByProjectAndStatus(r.Context(), db.ListScheduledEventsByProjectAndStatusParams{
			OrgID:     authCtx.OrgID,
			ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
			Status:    status,
			Limit:     int32(limit),
			Offset:    int32(offset),
		})
	} else {
		schedules, err = h.queries.ListScheduledEventsByProject(r.Context(), db.ListScheduledEventsByProjectParams{
			OrgID:     authCtx.OrgID,
			ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
			Limit:     int32(limit),
			Offset:    int32(offset),
		})
	}

	if err != nil {
		slog.Error("failed to list scheduled events", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list scheduled events"})
		return
	}

	// Convert to response
	result := make([]ScheduleResponse, len(schedules))
	for i, sch := range schedules {
		result[i] = scheduleToResponse(sch)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"schedules": result,
		"count":     len(result),
	})
}

// Get handles GET /schedules/:id.
func (h *SchedulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	sch, err := h.queries.GetScheduledEventByProject(r.Context(), db.GetScheduledEventByProjectParams{
		ID:        id,
		OrgID:     authCtx.OrgID,
		ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "scheduled event not found"})
		return
	}

	writeJSON(w, http.StatusOK, scheduleToResponse(sch))
}

// Cancel handles DELETE /schedules/:id.
func (h *SchedulesHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	rowsAffected, err := h.queries.CancelScheduledEventByProject(r.Context(), db.CancelScheduledEventByProjectParams{
		ID:        id,
		OrgID:     authCtx.OrgID,
		ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
	})
	if err != nil {
		slog.Error("failed to cancel scheduled event", "error", err, "id", id)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel scheduled event"})
		return
	}

	if rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "scheduled event not found or already executed"})
		return
	}

	slog.Info("scheduled event cancelled", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Run handles POST /schedules/:id/run.
func (h *SchedulesHandler) Run(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	eventID, err := h.scheduler.ExecuteNow(r.Context(), authCtx.OrgID, id)
	if err != nil {
		slog.Error("failed to execute scheduled event", "error", err, "id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "scheduled event not found or already executed"})
		return
	}

	slog.Info("scheduled event executed immediately", "id", id, "event_id", eventID)
	writeJSON(w, http.StatusOK, RunScheduleResponse{
		ScheduleID: id,
		EventID:    eventID,
	})
}

// Stats handles GET /stats/schedules.
func (h *SchedulesHandler) Stats(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	counts, err := h.queries.CountScheduledEventsByProjectStatus(r.Context(), db.CountScheduledEventsByProjectStatusParams{
		OrgID:     authCtx.OrgID,
		ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
	})
	if err != nil {
		slog.Error("failed to get schedule stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get schedule stats"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{
		"pending":   counts.Pending,
		"completed": counts.Completed,
		"cancelled": counts.Cancelled,
		"failed":    counts.Failed,
	})
}

func scheduleToResponse(sch db.ScheduledEvent) ScheduleResponse {
	resp := ScheduleResponse{
		ID:           sch.ID,
		Topic:        sch.Topic,
		Data:         sch.Data,
		ScheduledFor: sch.ScheduledFor.Time,
		Status:       sch.Status,
		CreatedAt:    sch.CreatedAt.Time,
	}
	if sch.Error.Valid {
		resp.Error = &sch.Error.String
	}
	if sch.ExecutedAt.Valid {
		resp.ExecutedAt = &sch.ExecutedAt.Time
	}
	return resp
}

func generateScheduleID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "sch_" + hex.EncodeToString(b)
}

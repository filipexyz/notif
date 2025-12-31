package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/filipexyz/notif/internal/nats"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	db   *pgxpool.Pool
	nats *nats.Client
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *pgxpool.Pool, nats *nats.Client) *HealthHandler {
	return &HealthHandler{db: db, nats: nats}
}

// Health is a simple liveness probe.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Ready is a readiness probe that checks dependencies.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status":   "ready",
		"nats":     "connected",
		"database": "connected",
	}

	status := http.StatusOK

	// Check NATS
	if !h.nats.IsConnected() {
		response["status"] = "not_ready"
		response["nats"] = "disconnected"
		status = http.StatusServiceUnavailable
	}

	// Check database
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		response["status"] = "not_ready"
		response["database"] = "disconnected"
		status = http.StatusServiceUnavailable
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

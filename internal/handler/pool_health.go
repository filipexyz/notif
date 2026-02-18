package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/filipexyz/notif/internal/nats"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolHealthHandler handles health check endpoints for multi-account mode.
type PoolHealthHandler struct {
	db   *pgxpool.Pool
	pool *nats.ClientPool
}

// NewPoolHealthHandler creates a new PoolHealthHandler.
func NewPoolHealthHandler(db *pgxpool.Pool, pool *nats.ClientPool) *PoolHealthHandler {
	return &PoolHealthHandler{db: db, pool: pool}
}

// Health is a simple liveness probe.
func (h *PoolHealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Ready is a readiness probe that checks dependencies.
func (h *PoolHealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]any{
		"status":   "ready",
		"nats":     "connected",
		"database": "connected",
		"accounts": h.pool.OrgCount(),
	}

	status := http.StatusOK

	// Check NATS pool health
	if !h.pool.IsHealthy() {
		response["status"] = "not_ready"
		disconnected := h.pool.DisconnectedOrgs()
		if len(disconnected) > 0 {
			response["nats"] = "partially_connected"
			response["disconnected_orgs"] = disconnected
		} else {
			response["nats"] = "disconnected"
		}
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

// Healthz is a detailed health endpoint for /healthz.
func (h *PoolHealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := http.StatusOK
	healthy := true

	// System connection
	systemStatus := "connected"
	if h.pool.SystemConn() == nil || !h.pool.SystemConn().IsConnected() {
		systemStatus = "disconnected"
		healthy = false
	}

	// Per-account connections
	orgStatuses := make(map[string]string)
	for _, orgID := range h.pool.OrgIDs() {
		client, err := h.pool.Get(orgID)
		if err != nil {
			orgStatuses[orgID] = "error"
			healthy = false
			continue
		}
		if client.IsConnected() {
			orgStatuses[orgID] = "connected"
		} else {
			orgStatuses[orgID] = "disconnected"
			healthy = false
		}
	}

	// Database
	dbStatus := "connected"
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		dbStatus = "disconnected"
		healthy = false
	}

	if !healthy {
		status = http.StatusServiceUnavailable
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"healthy":       healthy,
		"system_nats":   systemStatus,
		"database":      dbStatus,
		"accounts":      h.pool.OrgCount(),
		"account_status": orgStatuses,
	})
}

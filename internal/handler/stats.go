package handler

import (
	"net/http"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// StatsHandler handles stats endpoints.
type StatsHandler struct {
	queries     *db.Queries
	eventReader *nats.EventReader
	dlqReader   *nats.DLQReader
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(queries *db.Queries, eventReader *nats.EventReader, dlqReader *nats.DLQReader) *StatsHandler {
	return &StatsHandler{
		queries:     queries,
		eventReader: eventReader,
		dlqReader:   dlqReader,
	}
}

// OverviewResponse is the response for stats overview.
type OverviewResponse struct {
	Events   EventsOverview   `json:"events"`
	Webhooks WebhooksOverview `json:"webhooks"`
	DLQ      DLQOverview      `json:"dlq"`
	APIKeys  APIKeysOverview  `json:"api_keys"`
}

type EventsOverview struct {
	Total uint64 `json:"total"`
}

type WebhooksOverview struct {
	Total          int64   `json:"total"`
	Enabled        int64   `json:"enabled"`
	SuccessRate24h float64 `json:"success_rate_24h"`
}

type DLQOverview struct {
	Pending int64 `json:"pending"`
}

type APIKeysOverview struct {
	Total     int64 `json:"total"`
	Active24h int64 `json:"active_24h"`
}

// Overview returns dashboard overview stats.
func (h *StatsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "org_id required"})
		return
	}

	orgIDParam := pgtype.Text{String: authCtx.OrgID, Valid: true}

	resp := OverviewResponse{}

	// Events stats from database (project-scoped)
	if count, err := h.queries.CountEventsByProject(r.Context(), db.CountEventsByProjectParams{
		OrgID:     authCtx.OrgID,
		ProjectID: pgtype.Text{String: authCtx.ProjectID, Valid: authCtx.ProjectID != ""},
	}); err == nil {
		resp.Events.Total = uint64(count)
	}

	// Webhook stats
	if webhookStats, err := h.queries.GetWebhookStats(r.Context(), orgIDParam); err == nil {
		resp.Webhooks.Total = webhookStats.Total
		resp.Webhooks.Enabled = webhookStats.Enabled
	}

	// Webhook delivery stats for success rate
	if deliveryStats, err := h.queries.GetWebhookDeliveryStats(r.Context(), orgIDParam); err == nil {
		if deliveryStats.Total > 0 {
			resp.Webhooks.SuccessRate24h = float64(deliveryStats.SuccessCount) / float64(deliveryStats.Total)
		}
	}

	// DLQ stats (project-scoped)
	if dlqCount, err := h.dlqReader.Count(r.Context(), authCtx.OrgID, authCtx.ProjectID); err == nil {
		resp.DLQ.Pending = dlqCount
	}

	// API key stats
	if keyStats, err := h.queries.GetAPIKeyStats(r.Context(), orgIDParam); err == nil {
		resp.APIKeys.Total = keyStats.Total
		resp.APIKeys.Active24h = keyStats.Active24h
	}

	writeJSON(w, http.StatusOK, resp)
}

// EventsStatsResponse is the response for event stats.
type EventsStatsResponse struct {
	Stream StreamStats `json:"stream"`
}

type StreamStats struct {
	Messages  uint64 `json:"messages"`
	Bytes     uint64 `json:"bytes"`
	Consumers int    `json:"consumers"`
	FirstSeq  uint64 `json:"first_seq"`
	LastSeq   uint64 `json:"last_seq"`
}

// Events returns event/stream stats (org-scoped).
func (h *StatsHandler) Events(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "org_id required"})
		return
	}

	stats, err := h.queries.GetEventStats(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get event stats"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":    stats.Total,
		"last_24h": stats.Last24h,
		"last_1h":  stats.LastHour,
	})
}

// WebhooksStatsResponse is the response for webhook stats.
type WebhooksStatsResponse struct {
	Total       int64           `json:"total"`
	Enabled     int64           `json:"enabled"`
	Disabled    int64           `json:"disabled"`
	Deliveries  DeliveryStats   `json:"deliveries_24h"`
	ByWebhook   []WebhookStats  `json:"by_webhook"`
}

type DeliveryStats struct {
	Total       int64   `json:"total"`
	Success     int64   `json:"success"`
	Failed      int64   `json:"failed"`
	Pending     int64   `json:"pending"`
	SuccessRate float64 `json:"success_rate"`
}

type WebhookStats struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int32   `json:"avg_latency_ms"`
}

// Webhooks returns webhook stats.
func (h *StatsHandler) Webhooks(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.GetOrgIDFromContext(r.Context())
	if orgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "org_id required"})
		return
	}

	orgIDParam := pgtype.Text{String: orgID, Valid: true}

	resp := WebhooksStatsResponse{}

	// Webhook counts
	if webhookStats, err := h.queries.GetWebhookStats(r.Context(), orgIDParam); err == nil {
		resp.Total = webhookStats.Total
		resp.Enabled = webhookStats.Enabled
		resp.Disabled = webhookStats.Disabled
	}

	// Delivery stats
	if deliveryStats, err := h.queries.GetWebhookDeliveryStats(r.Context(), orgIDParam); err == nil {
		resp.Deliveries = DeliveryStats{
			Total:   deliveryStats.Total,
			Success: deliveryStats.SuccessCount,
			Failed:  deliveryStats.FailedCount,
			Pending: deliveryStats.PendingCount,
		}
		if deliveryStats.Total > 0 {
			resp.Deliveries.SuccessRate = float64(deliveryStats.SuccessCount) / float64(deliveryStats.Total)
		}
	}

	// Per-webhook stats
	if byWebhook, err := h.queries.GetWebhookDeliveryStatsByWebhook(r.Context(), orgIDParam); err == nil {
		resp.ByWebhook = make([]WebhookStats, len(byWebhook))
		for i, wh := range byWebhook {
			successRate := 0.0
			if wh.TotalDeliveries > 0 {
				successRate = float64(wh.SuccessCount) / float64(wh.TotalDeliveries)
			}
			resp.ByWebhook[i] = WebhookStats{
				ID:           uuid.UUID(wh.WebhookID.Bytes).String(),
				URL:          wh.Url,
				SuccessRate:  successRate,
				AvgLatencyMs: wh.AvgLatencyMs,
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// DLQStatsResponse is the response for DLQ stats.
type DLQStatsResponse struct {
	Total int64 `json:"total"`
}

// DLQ returns DLQ stats (project-scoped).
func (h *StatsHandler) DLQ(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "org_id required"})
		return
	}

	count, err := h.dlqReader.Count(r.Context(), authCtx.OrgID, authCtx.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get DLQ stats"})
		return
	}

	writeJSON(w, http.StatusOK, DLQStatsResponse{
		Total: count,
	})
}

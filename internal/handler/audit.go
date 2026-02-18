package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

// AuditHandler handles audit log queries.
type AuditHandler struct {
	queries *db.Queries
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(queries *db.Queries) *AuditHandler {
	return &AuditHandler{queries: queries}
}

// List returns audit log entries filtered by query parameters.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse query params
	actionFilter := r.URL.Query().Get("action")
	sinceStr := r.URL.Query().Get("since")

	limitStr := r.URL.Query().Get("limit")
	limit := int32(50)
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = int32(n)
		}
	}

	params := db.ListAuditLogsParams{
		Limit: limit,
	}

	// Enforce tenant isolation — OrgID is always required.
	// Without this, callers with no org scope could read all orgs' audit logs.
	if authCtx.OrgID == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "org scope required"})
		return
	}
	params.OrgID = pgtype.Text{String: authCtx.OrgID, Valid: true}

	if actionFilter != "" {
		params.Action = pgtype.Text{String: actionFilter, Valid: true}
	}

	if sinceStr != "" {
		if d, err := time.ParseDuration(sinceStr); err == nil {
			since := time.Now().Add(-d)
			params.Since = pgtype.Timestamptz{Time: since, Valid: true}
		}
	}

	logs, err := h.queries.ListAuditLogs(r.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query audit log"})
		return
	}

	// Format response
	results := make([]map[string]any, len(logs))
	for i, entry := range logs {
		result := map[string]any{
			"id":        entry.ID,
			"timestamp": entry.Timestamp.Time.Format(time.RFC3339),
			"actor":     entry.Actor,
			"action":    entry.Action,
		}
		if entry.OrgID.Valid {
			result["org_id"] = entry.OrgID.String
		}
		if entry.Target.Valid {
			result["target"] = entry.Target.String
		}
		if entry.Detail != nil {
			result["detail"] = json.RawMessage(entry.Detail)
		}
		if entry.IpAddress != nil {
			result["ip_address"] = entry.IpAddress.String()
		}
		results[i] = result
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": results,
		"count":   len(results),
	})
}

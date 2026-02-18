package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/filipexyz/notif/internal/accounts"
	"github.com/filipexyz/notif/internal/audit"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/go-chi/chi/v5"
)

var validOrgID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// OrgHandler handles org CRUD operations.
type OrgHandler struct {
	queries      *db.Queries
	pool         *nats.ClientPool
	accountMgr   *accounts.Manager
	auditLog     *audit.Logger
	onOrgCreated func(orgID string) // called after a new org is added to the pool
	onOrgDeleted func(orgID string) // called before an org is removed from the pool
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(queries *db.Queries, pool *nats.ClientPool, accountMgr *accounts.Manager, auditLog *audit.Logger) *OrgHandler {
	return &OrgHandler{
		queries:    queries,
		pool:       pool,
		accountMgr: accountMgr,
		auditLog:   auditLog,
	}
}

// SetOnOrgCreated sets a callback invoked after a new org is created and added to the pool.
// Used to start background workers (e.g., webhook delivery) for dynamically-created orgs.
func (h *OrgHandler) SetOnOrgCreated(fn func(orgID string)) {
	h.onOrgCreated = fn
}

// SetOnOrgDeleted sets a callback invoked before an org is removed from the pool.
// Used to stop background workers for the deleted org.
func (h *OrgHandler) SetOnOrgDeleted(fn func(orgID string)) {
	h.onOrgDeleted = fn
}

// CreateOrgRequest is the request body for creating an org.
type CreateOrgRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrgResponse is the response for an org.
type OrgResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	NatsPublicKey string `json:"nats_public_key"`
	BillingTier   string `json:"billing_tier"`
	CreatedAt     string `json:"created_at"`
}

// Create creates a new org with NATS account.
func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "multi-account mode not enabled",
		})
		return
	}

	var req CreateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.ID == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and name are required"})
		return
	}

	if !validOrgID.MatchString(req.ID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be 1-32 alphanumeric characters, hyphens, or underscores"})
		return
	}

	// Create org + NATS account
	result, err := h.accountMgr.CreateOrg(r.Context(), req.ID, req.Name)
	if err != nil {
		slog.Error("failed to create org", "org_id", req.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create org",
		})
		return
	}

	// Push JWT to NATS via system connection
	jwtMgr := h.accountMgr.JWTManager()
	if err := jwtMgr.RebuildAndPushAccountJWT(r.Context(), req.ID, h.pool.SystemConn()); err != nil {
		slog.Error("failed to push account JWT", "org_id", req.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to provision org account",
		})
		return
	}

	// Add to pool
	if err := h.pool.Add(r.Context(), req.ID, result.AccountKP); err != nil {
		slog.Error("failed to connect org to NATS pool", "org_id", req.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to connect org",
		})
		return
	}

	// Start background workers for the new org (webhook delivery, etc.)
	if h.onOrgCreated != nil {
		h.onOrgCreated(req.ID)
	}

	// Audit log
	if h.auditLog != nil {
		authCtx := middleware.GetAuthContext(r.Context())
		actor := auditActor(authCtx)
		ctx := audit.WithIP(r.Context(), audit.IPFromRequest(r))
		h.auditLog.Log(ctx, actor, "account.create", req.ID, req.ID, map[string]any{
			"name": req.Name,
		})
	}

	tier := "free"
	if result.Org.BillingTier.Valid {
		tier = result.Org.BillingTier.String
	}

	writeJSON(w, http.StatusCreated, OrgResponse{
		ID:            result.Org.ID,
		Name:          result.Org.Name,
		NatsPublicKey: result.Org.NatsPublicKey,
		BillingTier:   tier,
		CreatedAt:     result.Org.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// Delete deletes an org and revokes its NATS account.
func (h *OrgHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "multi-account mode not enabled",
		})
		return
	}

	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "org id required"})
		return
	}

	// Delete from DB first (cascades to projects, api_keys via FK)
	if err := h.accountMgr.DeleteOrg(r.Context(), orgID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to delete org",
		})
		return
	}

	// Stop background workers before disconnecting
	if h.onOrgDeleted != nil {
		h.onOrgDeleted(orgID)
	}

	// Then disconnect from NATS pool
	_ = h.pool.Remove(orgID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// List lists all orgs.
func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.queries.ListOrgs(r.Context())
	if err != nil {
		slog.Error("failed to list orgs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list orgs",
		})
		return
	}

	results := make([]OrgResponse, len(orgs))
	for i, org := range orgs {
		tier := "free"
		if org.BillingTier.Valid {
			tier = org.BillingTier.String
		}
		results[i] = OrgResponse{
			ID:            org.ID,
			Name:          org.Name,
			NatsPublicKey: org.NatsPublicKey,
			BillingTier:   tier,
			CreatedAt:     org.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"orgs":  results,
		"count": len(results),
	})
}

// UpdateLimitsRequest is the request body for updating org limits.
type UpdateLimitsRequest struct {
	BillingTier string `json:"billing_tier"`
}

// Limits shows or sets account limits for an org.
func (h *OrgHandler) Limits(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "id")
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "org id required"})
		return
	}

	if r.Method == http.MethodGet {
		org, err := h.queries.GetOrg(r.Context(), orgID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
			return
		}

		tier := "free"
		if org.BillingTier.Valid {
			tier = org.BillingTier.String
		}
		limits := accounts.DefaultTierLimits(tier)

		writeJSON(w, http.StatusOK, map[string]any{
			"org_id":       orgID,
			"billing_tier": tier,
			"limits": map[string]any{
				"max_connections": limits.MaxConnections,
				"max_data":        limits.MaxData,
				"max_payload":     limits.MaxPayload,
				"max_exports":     limits.MaxExports,
				"max_imports":     limits.MaxImports,
				"stream_max_age":  limits.StreamMaxAge.String(),
				"stream_max_bytes": limits.StreamMaxBytes,
			},
		})
		return
	}

	// PUT: update limits
	if h.pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "multi-account mode not enabled",
		})
		return
	}

	var req UpdateLimitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.BillingTier == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "billing_tier is required"})
		return
	}

	if !accounts.IsValidTier(req.BillingTier) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid billing_tier, must be: free, pro, or enterprise"})
		return
	}

	// Update billing tier in DB
	org, err := h.accountMgr.UpdateBillingTier(r.Context(), orgID, req.BillingTier)
	if err != nil {
		slog.Error("failed to update billing tier", "org_id", orgID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to update limits",
		})
		return
	}

	// Rebuild and push JWT with new limits
	jwtMgr := h.accountMgr.JWTManager()
	if err := jwtMgr.RebuildAndPushAccountJWT(r.Context(), orgID, h.pool.SystemConn()); err != nil {
		slog.Error("failed to push updated JWT", "org_id", orgID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to apply limits",
		})
		return
	}

	tier := "free"
	if org.BillingTier.Valid {
		tier = org.BillingTier.String
	}
	limits := accounts.DefaultTierLimits(tier)

	writeJSON(w, http.StatusOK, map[string]any{
		"org_id":       orgID,
		"billing_tier": tier,
		"limits": map[string]any{
			"max_connections": limits.MaxConnections,
			"max_data":        limits.MaxData,
			"max_payload":     limits.MaxPayload,
		},
	})
}

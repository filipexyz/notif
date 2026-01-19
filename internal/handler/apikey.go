package handler

import (
	"encoding/json"
	"net/http"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// APIKeyHandler handles API key management via Clerk-authenticated dashboard.
type APIKeyHandler struct {
	queries *db.Queries
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(queries *db.Queries) *APIKeyHandler {
	return &APIKeyHandler{queries: queries}
}

// CreateAPIKeyRequest is the request body for creating an API key.
type CreateAPIKeyRequest struct {
	Name      string `json:"name"`
	ProjectID string `json:"project_id,omitempty"` // Optional, defaults to current project
}

// APIKeyResponse is the response for an API key.
type APIKeyResponse struct {
	ID         string  `json:"id"`
	KeyPrefix  string  `json:"key_prefix"`
	FullKey    string  `json:"full_key,omitempty"` // Only returned on create
	Name       string  `json:"name,omitempty"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
}

// Create creates a new API key for the authenticated organization and project.
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.UserID == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Use project from request or current context
	projectID := req.ProjectID
	if projectID == "" {
		projectID = authCtx.ProjectID
	}
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}

	// Verify project belongs to org
	_, err := h.queries.GetProjectByOrgAndID(r.Context(), db.GetProjectByOrgAndIDParams{
		ID:    projectID,
		OrgID: authCtx.OrgID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project_id"})
		return
	}

	// Generate key
	fullKey, prefix, hash := domain.GenerateAPIKey()

	// Store with org_id and project_id
	apiKey, err := h.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		KeyHash:            hash,
		KeyPrefix:          prefix,
		Name:               pgtype.Text{String: req.Name, Valid: req.Name != ""},
		RateLimitPerSecond: pgtype.Int4{Int32: 100, Valid: true},
		OrgID:              pgtype.Text{String: authCtx.OrgID, Valid: true},
		ProjectID:          projectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create API key"})
		return
	}

	writeJSON(w, http.StatusCreated, APIKeyResponse{
		ID:        uuid.UUID(apiKey.ID.Bytes).String(),
		KeyPrefix: apiKey.KeyPrefix,
		FullKey:   fullKey, // Only returned once!
		Name:      apiKey.Name.String,
		CreatedAt: apiKey.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// List lists all API keys for the authenticated project.
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.UserID == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	keys, err := h.queries.ListAPIKeysByProject(r.Context(), db.ListAPIKeysByProjectParams{
		OrgID:     pgtype.Text{String: authCtx.OrgID, Valid: true},
		ProjectID: authCtx.ProjectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list API keys"})
		return
	}

	results := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		// Skip revoked keys
		if k.RevokedAt.Valid {
			continue
		}

		resp := APIKeyResponse{
			ID:        uuid.UUID(k.ID.Bytes).String(),
			KeyPrefix: k.KeyPrefix,
			Name:      k.Name.String,
			CreatedAt: k.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
		if k.LastUsedAt.Valid {
			t := k.LastUsedAt.Time.Format("2006-01-02T15:04:05Z")
			resp.LastUsedAt = &t
		}
		results = append(results, resp)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"api_keys": results,
		"count":    len(results),
	})
}

// Revoke revokes an API key (soft delete).
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.UserID == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid API key ID"})
		return
	}

	// Revoke only if key belongs to this project
	err = h.queries.RevokeAPIKeyByProject(r.Context(), db.RevokeAPIKeyByProjectParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		OrgID:     pgtype.Text{String: authCtx.OrgID, Valid: true},
		ProjectID: authCtx.ProjectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke API key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

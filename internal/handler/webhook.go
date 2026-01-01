package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// WebhookHandler handles webhook CRUD operations.
type WebhookHandler struct {
	queries *db.Queries
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(queries *db.Queries) *WebhookHandler {
	return &WebhookHandler{queries: queries}
}

// CreateWebhookRequest is the request body for creating a webhook.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Topics []string `json:"topics"`
}

// WebhookResponse is the response for a webhook.
type WebhookResponse struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Topics    []string `json:"topics"`
	Secret    string   `json:"secret,omitempty"` // Only returned on create
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at"`
}

// Create creates a new webhook.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	if len(req.Topics) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one topic is required"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Generate secret
	secret := generateSecret()

	webhook, err := h.queries.CreateWebhook(r.Context(), db.CreateWebhookParams{
		OrgID:  pgtype.Text{String: authCtx.OrgID, Valid: true},
		Url:    req.URL,
		Topics: req.Topics,
		Secret: secret,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create webhook"})
		return
	}

	writeJSON(w, http.StatusCreated, WebhookResponse{
		ID:        uuid.UUID(webhook.ID.Bytes).String(),
		URL:       webhook.Url,
		Topics:    webhook.Topics,
		Secret:    webhook.Secret, // Return secret only on create
		Enabled:   webhook.Enabled,
		CreatedAt: webhook.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// List lists all webhooks for the authenticated organization.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	webhooks, err := h.queries.GetWebhooksByOrg(r.Context(), pgtype.Text{String: authCtx.OrgID, Valid: true})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list webhooks"})
		return
	}

	results := make([]WebhookResponse, len(webhooks))
	for i, wh := range webhooks {
		results[i] = WebhookResponse{
			ID:        uuid.UUID(wh.ID.Bytes).String(),
			URL:       wh.Url,
			Topics:    wh.Topics,
			Enabled:   wh.Enabled,
			CreatedAt: wh.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"webhooks": results,
		"count":    len(results),
	})
}

// Get retrieves a specific webhook.
func (h *WebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook ID"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	webhook, err := h.queries.GetWebhook(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || webhook.OrgID.String != authCtx.OrgID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	writeJSON(w, http.StatusOK, WebhookResponse{
		ID:        uuid.UUID(webhook.ID.Bytes).String(),
		URL:       webhook.Url,
		Topics:    webhook.Topics,
		Enabled:   webhook.Enabled,
		CreatedAt: webhook.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// UpdateWebhookRequest is the request body for updating a webhook.
type UpdateWebhookRequest struct {
	URL     string   `json:"url"`
	Topics  []string `json:"topics"`
	Enabled *bool    `json:"enabled"`
}

// Update updates a webhook.
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook ID"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get existing webhook
	webhook, err := h.queries.GetWebhook(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || webhook.OrgID.String != authCtx.OrgID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Apply updates
	url := webhook.Url
	if req.URL != "" {
		url = req.URL
	}
	topics := webhook.Topics
	if len(req.Topics) > 0 {
		topics = req.Topics
	}
	enabled := webhook.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	updated, err := h.queries.UpdateWebhook(r.Context(), db.UpdateWebhookParams{
		ID:      webhook.ID,
		Url:     url,
		Topics:  topics,
		Enabled: enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update webhook"})
		return
	}

	writeJSON(w, http.StatusOK, WebhookResponse{
		ID:        uuid.UUID(updated.ID.Bytes).String(),
		URL:       updated.Url,
		Topics:    updated.Topics,
		Enabled:   updated.Enabled,
		CreatedAt: updated.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// Delete deletes a webhook.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook ID"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get existing webhook to verify ownership
	webhook, err := h.queries.GetWebhook(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || webhook.OrgID.String != authCtx.OrgID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	if err := h.queries.DeleteWebhook(r.Context(), webhook.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete webhook"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Deliveries lists recent deliveries for a webhook.
func (h *WebhookHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook ID"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Verify ownership
	webhook, err := h.queries.GetWebhook(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || webhook.OrgID.String != authCtx.OrgID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	deliveries, err := h.queries.GetWebhookDeliveries(r.Context(), db.GetWebhookDeliveriesParams{
		WebhookID: webhook.ID,
		Limit:     100,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get deliveries"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries": deliveries,
		"count":      len(deliveries),
	})
}

func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

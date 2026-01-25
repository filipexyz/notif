package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/filipexyz/notif/internal/config"
	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

// BootstrapHandler handles initial setup for self-hosted instances.
type BootstrapHandler struct {
	queries *db.Queries
	cfg     *config.Config
}

// NewBootstrapHandler creates a new BootstrapHandler.
func NewBootstrapHandler(queries *db.Queries, cfg *config.Config) *BootstrapHandler {
	return &BootstrapHandler{queries: queries, cfg: cfg}
}

// BootstrapResponse is returned when bootstrapping a new instance.
type BootstrapResponse struct {
	APIKey    string `json:"api_key"`
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`
}

// Bootstrap creates the initial API key for a self-hosted instance.
// This endpoint only works when:
// 1. AUTH_MODE=none (self-hosted mode)
// 2. No API keys exist in the database yet
func (h *BootstrapHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	// Only allow in self-hosted mode
	if !h.cfg.IsSelfHosted() {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "bootstrap only available in self-hosted mode (AUTH_MODE=none)",
		})
		return
	}

	// Check if any API keys already exist for this org
	keys, err := h.queries.ListAPIKeysByOrg(r.Context(), pgtype.Text{String: h.cfg.DefaultOrgID, Valid: true})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to check existing keys",
		})
		return
	}

	if len(keys) > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "instance already bootstrapped - API keys exist",
		})
		return
	}

	// Create default project first
	projectID := domain.GenerateProjectID()
	_, err = h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		ID:    projectID,
		OrgID: h.cfg.DefaultOrgID,
		Name:  "Default",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create default project",
		})
		return
	}

	// Generate a secure API key
	apiKey, err := generateBootstrapKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to generate api key",
		})
		return
	}

	// Hash the key for storage
	keyHash := sha256HashKey(apiKey)
	keyPrefix := apiKey[:16] // nsh_xxxxxxxxxxxx

	// Create the API key in the database
	_, err = h.queries.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Name:      pgtype.Text{String: "Bootstrap Key", Valid: true},
		OrgID:     pgtype.Text{String: h.cfg.DefaultOrgID, Valid: true},
		ProjectID: projectID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create api key",
		})
		return
	}

	writeJSON(w, http.StatusCreated, BootstrapResponse{
		APIKey:    apiKey,
		ProjectID: projectID,
		Message:   "Instance bootstrapped successfully. Save this API key - it won't be shown again!",
	})
}

// Status returns the bootstrap status of the instance.
func (h *BootstrapHandler) Status(w http.ResponseWriter, r *http.Request) {
	// Check if any API keys exist for default org
	keys, err := h.queries.ListAPIKeysByOrg(r.Context(), pgtype.Text{String: h.cfg.DefaultOrgID, Valid: true})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to check status",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bootstrapped": len(keys) > 0,
		"self_hosted":  h.cfg.IsSelfHosted(),
		"auth_mode":    string(h.cfg.AuthMode),
	})
}

func generateBootstrapKey() (string, error) {
	// Generate 28 random alphanumeric characters (32 total with nsh_ prefix)
	// Must match domain.ValidateKeyFormat regex: nsh_[a-zA-Z0-9]{28}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 28)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return "nsh_" + string(b), nil
}

func sha256HashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

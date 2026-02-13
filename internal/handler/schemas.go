package handler

import (
	"encoding/json"
	"net/http"

	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/go-chi/chi/v5"
)

// SchemaHandler handles schema-related HTTP requests.
type SchemaHandler struct {
	registry *schema.Registry
}

// NewSchemaHandler creates a new SchemaHandler.
func NewSchemaHandler(registry *schema.Registry) *SchemaHandler {
	return &SchemaHandler{registry: registry}
}

// CreateSchema handles POST /api/v1/schemas
func (h *SchemaHandler) CreateSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req schema.CreateSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.TopicPattern == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic_pattern is required"})
		return
	}

	s, err := h.registry.CreateSchema(ctx, auth.OrgID, auth.ProjectID, &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create schema"})
		return
	}

	writeJSON(w, http.StatusCreated, s)
}

// ListSchemas handles GET /api/v1/schemas
func (h *SchemaHandler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	schemas, err := h.registry.ListSchemas(ctx, auth.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list schemas"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"schemas": schemas,
		"count":   len(schemas),
	})
}

// GetSchema handles GET /api/v1/schemas/{name}
func (h *SchemaHandler) GetSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	s, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	writeJSON(w, http.StatusOK, s)
}

// UpdateSchema handles PUT /api/v1/schemas/{name}
func (h *SchemaHandler) UpdateSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	var req schema.UpdateSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	s, err := h.registry.UpdateSchema(ctx, existing.ID, &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update schema"})
		return
	}

	writeJSON(w, http.StatusOK, s)
}

// DeleteSchema handles DELETE /api/v1/schemas/{name}
func (h *SchemaHandler) DeleteSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	if err := h.registry.DeleteSchema(ctx, existing.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete schema"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// CreateVersion handles POST /api/v1/schemas/{name}/versions
func (h *SchemaHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	var req schema.CreateSchemaVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Version == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version is required"})
		return
	}
	if len(req.Schema) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schema is required"})
		return
	}

	createdBy := ""
	if auth.UserID != nil {
		createdBy = *auth.UserID
	}
	v, err := h.registry.CreateVersion(ctx, existing.ID, &req, createdBy)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to create schema version"})
		return
	}

	writeJSON(w, http.StatusCreated, v)
}

// ListVersions handles GET /api/v1/schemas/{name}/versions
func (h *SchemaHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	versions, err := h.registry.ListVersions(ctx, existing.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list versions"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"versions": versions,
		"count":    len(versions),
	})
}

// GetVersion handles GET /api/v1/schemas/{name}/versions/{version}
func (h *SchemaHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	if name == "" || version == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and version are required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	v, err := h.registry.GetVersion(ctx, existing.ID, version)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "version not found"})
		return
	}

	writeJSON(w, http.StatusOK, v)
}

// Validate handles POST /api/v1/schemas/{name}/validate
func (h *SchemaHandler) Validate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Get existing schema
	existing, err := h.registry.GetSchemaByName(ctx, auth.ProjectID, name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schema not found"})
		return
	}

	var req schema.ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	result, err := h.registry.Validate(ctx, existing.ID, req.Data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "validation failed"})
		return
	}

	result.Schema = existing.Name
	writeJSON(w, http.StatusOK, result)
}

// GetSchemaForTopic handles GET /api/v1/schemas/for-topic/{topic}
func (h *SchemaHandler) GetSchemaForTopic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth := middleware.GetAuthContext(ctx)
	if auth == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	topic := chi.URLParam(r, "topic")
	if topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic is required"})
		return
	}

	s, err := h.registry.GetSchemaForTopic(ctx, auth.ProjectID, topic)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get schema for topic"})
		return
	}

	if s == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no schema found for topic"})
		return
	}

	writeJSON(w, http.StatusOK, s)
}

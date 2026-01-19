package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/filipexyz/notif/internal/db"
	"github.com/filipexyz/notif/internal/domain"
	"github.com/filipexyz/notif/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// ProjectHandler handles project CRUD operations.
type ProjectHandler struct {
	queries *db.Queries
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(queries *db.Queries) *ProjectHandler {
	return &ProjectHandler{queries: queries}
}

// CreateProjectRequest is the request body for creating a project.
type CreateProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

// UpdateProjectRequest is the request body for updating a project.
type UpdateProjectRequest struct {
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

// ProjectResponse is the response for a project.
type ProjectResponse struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// toSlug converts a name to a URL-safe slug.
func toSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

// Create creates a new project.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Generate slug from name if not provided
	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	}

	// Validate slug format
	if !slugRegex.MatchString(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "slug must be lowercase alphanumeric with hyphens only",
		})
		return
	}

	project, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		ID:    domain.GenerateProjectID(),
		OrgID: authCtx.OrgID,
		Name:  req.Name,
		Slug:  slug,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "project with this slug already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create project"})
		return
	}

	writeJSON(w, http.StatusCreated, ProjectResponse{
		ID:        project.ID,
		OrgID:     project.OrgID,
		Name:      project.Name,
		Slug:      project.Slug,
		CreatedAt: project.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: project.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// List lists all projects for the authenticated org.
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projects, err := h.queries.ListProjectsByOrg(r.Context(), authCtx.OrgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}

	resp := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = ProjectResponse{
			ID:        p.ID,
			OrgID:     p.OrgID,
			Name:      p.Name,
			Slug:      p.Slug,
			CreatedAt: p.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: p.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects": resp,
		"count":    len(resp),
	})
}

// Get retrieves a project by ID.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	project, err := h.queries.GetProjectByOrgAndID(r.Context(), db.GetProjectByOrgAndIDParams{
		ID:    id,
		OrgID: authCtx.OrgID,
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	writeJSON(w, http.StatusOK, ProjectResponse{
		ID:        project.ID,
		OrgID:     project.OrgID,
		Name:      project.Name,
		Slug:      project.Slug,
		CreatedAt: project.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: project.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// Update updates a project.
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate slug if provided
	if req.Slug != "" && !slugRegex.MatchString(req.Slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "slug must be lowercase alphanumeric with hyphens only",
		})
		return
	}

	project, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:      id,
		OrgID:   authCtx.OrgID,
		Column3: req.Name, // name
		Column4: req.Slug, // slug
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "project with this slug already exists"})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	writeJSON(w, http.StatusOK, ProjectResponse{
		ID:        project.ID,
		OrgID:     project.OrgID,
		Name:      project.Name,
		Slug:      project.Slug,
		CreatedAt: project.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: project.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	})
}

// Delete deletes a project.
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	authCtx := middleware.GetAuthContext(r.Context())
	if authCtx == nil || authCtx.OrgID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	// Prevent deleting the default project
	project, err := h.queries.GetProjectByOrgAndID(r.Context(), db.GetProjectByOrgAndIDParams{
		ID:    id,
		OrgID: authCtx.OrgID,
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if project.Slug == "default" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete default project"})
		return
	}

	if err := h.queries.DeleteProject(r.Context(), db.DeleteProjectParams{
		ID:    id,
		OrgID: authCtx.OrgID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete project"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

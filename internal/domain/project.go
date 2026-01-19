package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Project represents a project within an organization.
// All resources (events, webhooks, schedules, API keys) are scoped to a project.
type Project struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GenerateProjectID creates a unique project ID with "prj_" prefix.
func GenerateProjectID() string {
	b := make([]byte, 14)
	rand.Read(b)
	return "prj_" + hex.EncodeToString(b)[:27]
}

// CreateProjectRequest is the request body for creating a project.
type CreateProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// UpdateProjectRequest is the request body for updating a project.
type UpdateProjectRequest struct {
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
}

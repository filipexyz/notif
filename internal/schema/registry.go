package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	DefaultRegistryURL = "https://raw.githubusercontent.com/notifsh/schemas/main"
	NamespacesURL      = DefaultRegistryURL + "/namespaces.json"
)

// Registry provides access to the schema registry
type Registry struct {
	baseURL string
	client  *http.Client
}

// NewRegistry creates a new registry client
func NewRegistry() *Registry {
	return &Registry{
		baseURL: DefaultRegistryURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SchemaRef represents a schema reference (namespace/name@version)
type SchemaRef struct {
	Namespace string
	Name      string
	Version   string
}

// String returns the string representation of the schema reference
func (r *SchemaRef) String() string {
	if r.Version == "latest" || r.Version == "" {
		return fmt.Sprintf("@%s/%s", r.Namespace, r.Name)
	}
	return fmt.Sprintf("@%s/%s@%s", r.Namespace, r.Name, r.Version)
}

// parseRef parses a schema reference string
// Examples: @filipelabs/agent, @filipelabs/agent@1.0.0
func ParseSchemaRef(ref string) (*SchemaRef, error) {
	if !strings.HasPrefix(ref, "@") {
		return nil, fmt.Errorf("invalid schema reference: must start with @ (e.g., @namespace/name)")
	}

	ref = strings.TrimPrefix(ref, "@")
	parts := strings.Split(ref, "@")

	var namePart, version string
	if len(parts) == 1 {
		namePart = parts[0]
		version = "latest"
	} else if len(parts) == 2 {
		namePart = parts[0]
		version = parts[1]
	} else {
		return nil, fmt.Errorf("invalid schema reference format")
	}

	nameComponents := strings.Split(namePart, "/")
	if len(nameComponents) != 2 {
		return nil, fmt.Errorf("invalid schema reference: must be in format @namespace/name")
	}

	return &SchemaRef{
		Namespace: nameComponents[0],
		Name:      nameComponents[1],
		Version:   version,
	}, nil
}

// RegistrySchema represents a schema in the registry
type RegistrySchema struct {
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"published_at"`
}

// RegistryIndex represents the index of all schemas in the registry
type RegistryIndex struct {
	Schemas map[string][]RegistrySchema `json:"schemas"` // key: namespace/name
}

// Fetch fetches a schema from the registry
func (r *Registry) Fetch(ref *SchemaRef) (*Schema, error) {
	version := ref.Version
	if version == "latest" {
		v, err := r.GetLatestVersion(ref.Namespace, ref.Name)
		if err != nil {
			return nil, err
		}
		version = v
	}

	// Validate version is semver
	if _, err := semver.NewVersion(version); err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}

	url := fmt.Sprintf("%s/schemas/%s/%s/%s/schema.json",
		r.baseURL, ref.Namespace, ref.Name, version)

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("schema not found: %s", ref.String())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch schema: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jsonSchema map[string]interface{}
	if err := json.Unmarshal(body, &jsonSchema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	// Extract metadata from JSON Schema
	schema := &Schema{
		Name:        ref.Name,
		Version:     version,
		Description: extractString(jsonSchema, "description"),
		Fields:      make(map[string]*Field),
	}

	// Extract fields from properties
	if properties, ok := jsonSchema["properties"].(map[string]interface{}); ok {
		required := extractStringArray(jsonSchema, "required")
		requiredMap := make(map[string]bool)
		for _, r := range required {
			requiredMap[r] = true
		}

		for fieldName, fieldData := range properties {
			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				schema.Fields[fieldName] = &Field{
					Type:        extractString(fieldMap, "type"),
					Required:    requiredMap[fieldName],
					Description: extractString(fieldMap, "description"),
					Default:     fieldMap["default"],
				}
			}
		}
	}

	return schema, nil
}

// GetLatestVersion returns the latest version for a schema
func (r *Registry) GetLatestVersion(namespace, name string) (string, error) {
	versions, err := r.ListVersions(namespace, name)
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for @%s/%s", namespace, name)
	}

	// Parse all versions as semver
	semvers := make([]*semver.Version, 0, len(versions))
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue // Skip invalid versions
		}
		semvers = append(semvers, sv)
	}

	if len(semvers) == 0 {
		return "", fmt.Errorf("no valid semver versions found")
	}

	// Find the latest version
	latest := semvers[0]
	for _, sv := range semvers[1:] {
		if sv.GreaterThan(latest) {
			latest = sv
		}
	}

	return latest.String(), nil
}

// ListVersions lists all available versions for a schema
func (r *Registry) ListVersions(namespace, name string) ([]string, error) {
	// Fetch the index file for this schema
	url := fmt.Sprintf("%s/schemas/%s/%s/index.json", r.baseURL, namespace, name)

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("schema not found: @%s/%s", namespace, name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch versions: HTTP %d", resp.StatusCode)
	}

	var index struct {
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}

	return index.Versions, nil
}

// Search searches for schemas matching a query
func (r *Registry) Search(query string, opts SearchOptions) ([]RegistrySchema, error) {
	// Fetch the registry index
	resp, err := r.client.Get(r.baseURL + "/index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch registry index: HTTP %d", resp.StatusCode)
	}

	var index RegistryIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to parse registry index: %w", err)
	}

	// Filter schemas
	results := make([]RegistrySchema, 0)
	queryLower := strings.ToLower(query)

	for key, schemas := range index.Schemas {
		// Filter by namespace if specified
		if opts.Namespace != "" {
			parts := strings.Split(key, "/")
			if len(parts) != 2 || parts[0] != opts.Namespace {
				continue
			}
		}

		// Get the latest version for each schema
		if len(schemas) == 0 {
			continue
		}

		latest := schemas[0]
		for _, s := range schemas[1:] {
			v1, err1 := semver.NewVersion(s.Version)
			v2, err2 := semver.NewVersion(latest.Version)
			if err1 == nil && err2 == nil && v1.GreaterThan(v2) {
				latest = s
			}
		}

		// Filter by query
		if query != "" {
			nameLower := strings.ToLower(latest.Name)
			descLower := strings.ToLower(latest.Description)
			nsLower := strings.ToLower(latest.Namespace)

			if !strings.Contains(nameLower, queryLower) &&
				!strings.Contains(descLower, queryLower) &&
				!strings.Contains(nsLower, queryLower) {
				continue
			}
		}

		results = append(results, latest)

		if opts.Limit > 0 && len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// SearchOptions contains options for searching schemas
type SearchOptions struct {
	Namespace string
	Limit     int
}

// GetSchemaInfo returns detailed information about a schema
func (r *Registry) GetSchemaInfo(ref *SchemaRef) (*SchemaInfo, error) {
	// List all versions
	versions, err := r.ListVersions(ref.Namespace, ref.Name)
	if err != nil {
		return nil, err
	}

	// Fetch the latest version schema
	latest, err := r.GetLatestVersion(ref.Namespace, ref.Name)
	if err != nil {
		return nil, err
	}

	latestRef := &SchemaRef{
		Namespace: ref.Namespace,
		Name:      ref.Name,
		Version:   latest,
	}

	schema, err := r.Fetch(latestRef)
	if err != nil {
		return nil, err
	}

	// Fetch README if available
	readmeURL := fmt.Sprintf("%s/schemas/%s/%s/%s/README.md",
		r.baseURL, ref.Namespace, ref.Name, latest)

	readme := ""
	if resp, err := r.client.Get(readmeURL); err == nil && resp.StatusCode == http.StatusOK {
		if body, err := io.ReadAll(resp.Body); err == nil {
			readme = string(body)
		}
		resp.Body.Close()
	}

	return &SchemaInfo{
		Namespace:   ref.Namespace,
		Name:        ref.Name,
		Description: schema.Description,
		Latest:      latest,
		Versions:    versions,
		Schema:      schema,
		README:      readme,
	}, nil
}

// SchemaInfo contains detailed information about a schema
type SchemaInfo struct {
	Namespace   string
	Name        string
	Description string
	Latest      string
	Versions    []string
	Schema      *Schema
	README      string
}

// CheckNamespaceAvailable checks if a namespace is available
func (r *Registry) CheckNamespaceAvailable(namespace string) (bool, error) {
	resp, err := r.client.Get(NamespacesURL)
	if err != nil {
		return false, fmt.Errorf("failed to fetch namespaces: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return true, nil // No namespaces file = all available
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to fetch namespaces: HTTP %d", resp.StatusCode)
	}

	var namespaces map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&namespaces); err != nil {
		return false, fmt.Errorf("failed to parse namespaces: %w", err)
	}

	_, exists := namespaces[namespace]
	return !exists, nil
}

// CheckVersionExists checks if a version already exists for a schema
func (r *Registry) CheckVersionExists(namespace, name, version string) (bool, error) {
	versions, err := r.ListVersions(namespace, name)
	if err != nil {
		// If schema doesn't exist, version doesn't exist
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	for _, v := range versions {
		if v == version {
			return true, nil
		}
	}

	return false, nil
}

// Helper functions
func extractString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func extractStringArray(m map[string]interface{}, key string) []string {
	if arr, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

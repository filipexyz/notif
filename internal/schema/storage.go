package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Storage handles local schema storage.
type Storage struct {
	basePath string
}

// NewStorage creates a new Storage instance.
func NewStorage() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	basePath := filepath.Join(home, ".notif", "schemas")
	return &Storage{basePath: basePath}, nil
}

// NewStorageWithPath creates a new Storage instance with a custom base path.
func NewStorageWithPath(basePath string) *Storage {
	return &Storage{basePath: basePath}
}

// Save saves a schema to local storage.
func (s *Storage) Save(namespace, name, version string, schema *Schema, jsonSchema *JSONSchema) error {
	schemaDir := s.getSchemaPath(namespace, name, version)

	// Create directory
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Save original schema (YAML format)
	schemaData, err := MarshalYAML(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	schemaFile := filepath.Join(schemaDir, "schema.yaml")
	if err := os.WriteFile(schemaFile, schemaData, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	// Save JSON Schema
	jsonSchemaData, err := json.MarshalIndent(jsonSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON schema: %w", err)
	}
	jsonSchemaFile := filepath.Join(schemaDir, "schema.json")
	if err := os.WriteFile(jsonSchemaFile, jsonSchemaData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON schema file: %w", err)
	}

	// Save metadata
	meta := &InstalledSchema{
		Namespace:   namespace,
		Name:        name,
		Version:     version,
		Description: schema.Description,
		InstalledAt: time.Now(),
		Path:        schemaDir,
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	metaFile := filepath.Join(schemaDir, "meta.json")
	if err := os.WriteFile(metaFile, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Load loads a schema from local storage.
func (s *Storage) Load(namespace, name, version string) (*Schema, *JSONSchema, *InstalledSchema, error) {
	schemaDir := s.getSchemaPath(namespace, name, version)

	// Load metadata
	metaFile := filepath.Join(schemaDir, "meta.json")
	metaData, err := os.ReadFile(metaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil, fmt.Errorf("schema not found")
		}
		return nil, nil, nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta InstalledSchema
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Load original schema
	schemaFile := filepath.Join(schemaDir, "schema.yaml")
	schema, err := Parse(schemaFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load schema: %w", err)
	}

	// Load JSON Schema
	jsonSchemaFile := filepath.Join(schemaDir, "schema.json")
	jsonSchemaData, err := os.ReadFile(jsonSchemaFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read JSON schema: %w", err)
	}

	var jsonSchema JSONSchema
	if err := json.Unmarshal(jsonSchemaData, &jsonSchema); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse JSON schema: %w", err)
	}

	return schema, &jsonSchema, &meta, nil
}

// List lists all installed schemas.
func (s *Storage) List() ([]*InstalledSchema, error) {
	var schemas []*InstalledSchema

	// Check if base path exists
	if _, err := os.Stat(s.basePath); os.IsNotExist(err) {
		return schemas, nil
	}

	// Walk through namespaces
	namespaces, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schemas directory: %w", err)
	}

	for _, nsEntry := range namespaces {
		if !nsEntry.IsDir() {
			continue
		}

		namespace := nsEntry.Name()
		nsPath := filepath.Join(s.basePath, namespace)

		// Walk through schema names
		names, err := os.ReadDir(nsPath)
		if err != nil {
			continue
		}

		for _, nameEntry := range names {
			if !nameEntry.IsDir() {
				continue
			}

			name := nameEntry.Name()
			namePath := filepath.Join(nsPath, name)

			// Walk through versions
			versions, err := os.ReadDir(namePath)
			if err != nil {
				continue
			}

			for _, versionEntry := range versions {
				if !versionEntry.IsDir() {
					continue
				}

				version := versionEntry.Name()

				// Load metadata
				metaFile := filepath.Join(namePath, version, "meta.json")
				metaData, err := os.ReadFile(metaFile)
				if err != nil {
					continue
				}

				var meta InstalledSchema
				if err := json.Unmarshal(metaData, &meta); err != nil {
					continue
				}

				schemas = append(schemas, &meta)
			}
		}
	}

	// Sort by installed time (most recent first)
	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].InstalledAt.After(schemas[j].InstalledAt)
	})

	return schemas, nil
}

// Remove removes a specific version of a schema.
func (s *Storage) Remove(namespace, name, version string) error {
	schemaDir := s.getSchemaPath(namespace, name, version)

	if _, err := os.Stat(schemaDir); os.IsNotExist(err) {
		return fmt.Errorf("schema not found")
	}

	if err := os.RemoveAll(schemaDir); err != nil {
		return fmt.Errorf("failed to remove schema: %w", err)
	}

	// Clean up empty parent directories
	s.cleanupEmptyDirs(namespace, name)

	return nil
}

// RemoveAll removes all versions of a schema.
func (s *Storage) RemoveAll(namespace, name string) error {
	nameDir := filepath.Join(s.basePath, namespace, name)

	if _, err := os.Stat(nameDir); os.IsNotExist(err) {
		return fmt.Errorf("schema not found")
	}

	if err := os.RemoveAll(nameDir); err != nil {
		return fmt.Errorf("failed to remove schema: %w", err)
	}

	// Clean up empty parent directories
	s.cleanupEmptyDirs(namespace, "")

	return nil
}

// ListVersions lists all installed versions of a schema.
func (s *Storage) ListVersions(namespace, name string) ([]string, error) {
	nameDir := filepath.Join(s.basePath, namespace, name)

	if _, err := os.Stat(nameDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(nameDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}

	return versions, nil
}

// Exists checks if a schema exists.
func (s *Storage) Exists(namespace, name, version string) bool {
	schemaDir := s.getSchemaPath(namespace, name, version)
	_, err := os.Stat(schemaDir)
	return err == nil
}

// getSchemaPath returns the path for a specific schema version.
func (s *Storage) getSchemaPath(namespace, name, version string) string {
	return filepath.Join(s.basePath, namespace, name, version)
}

// cleanupEmptyDirs removes empty parent directories.
func (s *Storage) cleanupEmptyDirs(namespace, name string) {
	if name != "" {
		// Try to remove name directory if empty
		nameDir := filepath.Join(s.basePath, namespace, name)
		if isEmpty(nameDir) {
			os.Remove(nameDir)
		}
	}

	// Try to remove namespace directory if empty
	nsDir := filepath.Join(s.basePath, namespace)
	if isEmpty(nsDir) {
		os.Remove(nsDir)
	}
}

// isEmpty checks if a directory is empty.
func isEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}

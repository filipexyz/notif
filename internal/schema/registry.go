package schema

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/filipexyz/notif/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// Registry manages schemas and provides validation services.
type Registry struct {
	queries   *db.Queries
	validator *Validator

	// Cache for schema lookups by topic
	topicCache sync.Map // map[projectID:topic]*SchemaVersion
}

// NewRegistry creates a new schema registry.
func NewRegistry(queries *db.Queries) *Registry {
	return &Registry{
		queries:   queries,
		validator: NewValidator(),
	}
}

// CreateSchema creates a new schema.
func (r *Registry) CreateSchema(ctx context.Context, orgID, projectID string, req *CreateSchemaRequest) (*Schema, error) {
	id := generateSchemaID()

	dbSchema, err := r.queries.CreateSchema(ctx, db.CreateSchemaParams{
		ID:           id,
		OrgID:        orgID,
		ProjectID:    projectID,
		Name:         req.Name,
		TopicPattern: req.TopicPattern,
		Description:  pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Tags:         req.Tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return dbSchemaToSchema(dbSchema), nil
}

// GetSchema retrieves a schema by ID.
func (r *Registry) GetSchema(ctx context.Context, id string) (*Schema, error) {
	dbSchema, err := r.queries.GetSchema(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("schema not found: %w", err)
	}

	schema := dbSchemaToSchema(dbSchema)

	// Load latest version
	latestVersion, err := r.queries.GetLatestSchemaVersion(ctx, id)
	if err == nil {
		schema.LatestVersion = dbVersionToVersion(latestVersion)
	}

	return schema, nil
}

// GetSchemaByName retrieves a schema by project and name.
func (r *Registry) GetSchemaByName(ctx context.Context, projectID, name string) (*Schema, error) {
	dbSchema, err := r.queries.GetSchemaByName(ctx, db.GetSchemaByNameParams{
		ProjectID: projectID,
		Name:      name,
	})
	if err != nil {
		return nil, fmt.Errorf("schema not found: %w", err)
	}

	schema := dbSchemaToSchema(dbSchema)

	// Load latest version
	latestVersion, err := r.queries.GetLatestSchemaVersion(ctx, dbSchema.ID)
	if err == nil {
		schema.LatestVersion = dbVersionToVersion(latestVersion)
	}

	return schema, nil
}

// ListSchemas lists all schemas in a project.
func (r *Registry) ListSchemas(ctx context.Context, projectID string) ([]*Schema, error) {
	dbSchemas, err := r.queries.ListSchemas(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	schemas := make([]*Schema, len(dbSchemas))
	for i, dbs := range dbSchemas {
		schemas[i] = dbSchemaToSchema(dbs)

		// Load latest version for each
		latestVersion, err := r.queries.GetLatestSchemaVersion(ctx, dbs.ID)
		if err == nil {
			schemas[i].LatestVersion = dbVersionToVersion(latestVersion)
		}
	}

	return schemas, nil
}

// UpdateSchema updates a schema's metadata.
func (r *Registry) UpdateSchema(ctx context.Context, id string, req *UpdateSchemaRequest) (*Schema, error) {
	existing, err := r.queries.GetSchema(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("schema not found: %w", err)
	}

	topicPattern := existing.TopicPattern
	if req.TopicPattern != "" {
		topicPattern = req.TopicPattern
	}

	description := existing.Description.String
	if req.Description != "" {
		description = req.Description
	}

	tags := existing.Tags
	if req.Tags != nil {
		tags = req.Tags
	}

	dbSchema, err := r.queries.UpdateSchema(ctx, db.UpdateSchemaParams{
		ID:           id,
		TopicPattern: topicPattern,
		Description:  pgtype.Text{String: description, Valid: description != ""},
		Tags:         tags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update schema: %w", err)
	}

	// Invalidate cache
	r.invalidateTopicCache(existing.ProjectID)

	return dbSchemaToSchema(dbSchema), nil
}

// DeleteSchema deletes a schema and all its versions.
func (r *Registry) DeleteSchema(ctx context.Context, id string) error {
	existing, err := r.queries.GetSchema(ctx, id)
	if err != nil {
		return fmt.Errorf("schema not found: %w", err)
	}

	if err := r.queries.DeleteSchema(ctx, id); err != nil {
		return fmt.Errorf("failed to delete schema: %w", err)
	}

	// Invalidate cache
	r.invalidateTopicCache(existing.ProjectID)

	return nil
}

// CreateVersion creates a new version of a schema.
func (r *Registry) CreateVersion(ctx context.Context, schemaID string, req *CreateSchemaVersionRequest, createdBy string) (*SchemaVersion, error) {
	// Validate the JSON schema itself
	if err := IsValidSchema(req.Schema); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	// Get the schema to find project ID
	schema, err := r.queries.GetSchema(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("schema not found: %w", err)
	}

	// Set defaults
	validationMode := req.ValidationMode
	if validationMode == "" {
		validationMode = ValidationModeStrict
	}

	onInvalid := req.OnInvalid
	if onInvalid == "" {
		onInvalid = OnInvalidReject
	}

	compatibility := req.Compatibility
	if compatibility == "" {
		compatibility = CompatibilityBackward
	}

	// Compute fingerprint
	fingerprint := Fingerprint(req.Schema)

	id := generateVersionID()

	// Set all versions to not latest, then create the new one as latest
	if err := r.queries.SetSchemaVersionLatest(ctx, db.SetSchemaVersionLatestParams{
		SchemaID: schemaID,
		ID:       "", // No ID matches, so all become false
	}); err != nil {
		return nil, fmt.Errorf("failed to update versions: %w", err)
	}

	dbVersion, err := r.queries.CreateSchemaVersion(ctx, db.CreateSchemaVersionParams{
		ID:             id,
		SchemaID:       schemaID,
		Version:        req.Version,
		SchemaJson:     req.Schema,
		ValidationMode: pgtype.Text{String: string(validationMode), Valid: true},
		OnInvalid:      pgtype.Text{String: string(onInvalid), Valid: true},
		Compatibility:  pgtype.Text{String: string(compatibility), Valid: true},
		Examples:       req.Examples,
		Fingerprint:    pgtype.Text{String: fingerprint, Valid: true},
		IsLatest:       pgtype.Bool{Bool: true, Valid: true},
		CreatedBy:      pgtype.Text{String: createdBy, Valid: createdBy != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	// Invalidate cache
	r.invalidateTopicCache(schema.ProjectID)

	return dbVersionToVersion(dbVersion), nil
}

// GetVersion retrieves a specific version.
func (r *Registry) GetVersion(ctx context.Context, schemaID, version string) (*SchemaVersion, error) {
	dbVersion, err := r.queries.GetSchemaVersionByVersion(ctx, db.GetSchemaVersionByVersionParams{
		SchemaID: schemaID,
		Version:  version,
	})
	if err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}

	return dbVersionToVersion(dbVersion), nil
}

// ListVersions lists all versions of a schema.
func (r *Registry) ListVersions(ctx context.Context, schemaID string) ([]*SchemaVersion, error) {
	dbVersions, err := r.queries.ListSchemaVersions(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	versions := make([]*SchemaVersion, len(dbVersions))
	for i, v := range dbVersions {
		versions[i] = dbVersionToVersion(v)
	}

	return versions, nil
}

// GetSchemaForTopic finds the schema that matches a topic.
func (r *Registry) GetSchemaForTopic(ctx context.Context, projectID, topic string) (*Schema, error) {
	// Check cache first
	cacheKey := projectID + ":" + topic
	if cached, ok := r.topicCache.Load(cacheKey); ok {
		if cached == nil {
			return nil, nil // Cached "no schema"
		}
		return cached.(*Schema), nil
	}

	// Query database
	dbSchema, err := r.queries.GetSchemaForTopic(ctx, db.GetSchemaForTopicParams{
		ProjectID: projectID,
		Topic:     topic,
	})
	if err != nil {
		// Cache the miss
		r.topicCache.Store(cacheKey, nil)
		return nil, nil
	}

	schema := dbSchemaToSchema(dbSchema)

	// Load latest version
	latestVersion, err := r.queries.GetLatestSchemaVersion(ctx, dbSchema.ID)
	if err == nil {
		schema.LatestVersion = dbVersionToVersion(latestVersion)
	}

	// Cache the result
	r.topicCache.Store(cacheKey, schema)

	return schema, nil
}

// ValidateEvent validates event data against the schema for its topic.
func (r *Registry) ValidateEvent(ctx context.Context, projectID, topic string, data json.RawMessage) (*ValidationResult, error) {
	schema, err := r.GetSchemaForTopic(ctx, projectID, topic)
	if err != nil {
		return nil, err
	}

	if schema == nil || schema.LatestVersion == nil {
		// No schema for this topic - pass through
		return &ValidationResult{Valid: true}, nil
	}

	result, err := r.validator.ValidateWithVersion(schema.LatestVersion, data)
	if err != nil {
		return nil, err
	}

	result.Schema = schema.Name
	return result, nil
}

// Validate validates data against a specific schema.
func (r *Registry) Validate(ctx context.Context, schemaID string, data json.RawMessage) (*ValidationResult, error) {
	latestVersion, err := r.queries.GetLatestSchemaVersion(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("no version found for schema: %w", err)
	}

	sv := dbVersionToVersion(latestVersion)
	return r.validator.ValidateWithVersion(sv, data)
}

func (r *Registry) invalidateTopicCache(projectID string) {
	// Simple approach: clear all entries for this project
	r.topicCache.Range(func(key, value interface{}) bool {
		if strings.HasPrefix(key.(string), projectID+":") {
			r.topicCache.Delete(key)
		}
		return true
	})
}

// Helper functions

func generateSchemaID() string {
	return "sch_" + generateRandomID(24)
}

func generateVersionID() string {
	return "schv_" + generateRandomID(24)
}

func generateRandomID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	randomBytes := make([]byte, length)
	_, _ = rand.Read(randomBytes)
	for i := range b {
		b[i] = charset[int(randomBytes[i])%len(charset)]
	}
	return string(b)
}

func dbSchemaToSchema(dbs db.Schema) *Schema {
	return &Schema{
		ID:           dbs.ID,
		OrgID:        dbs.OrgID,
		ProjectID:    dbs.ProjectID,
		Name:         dbs.Name,
		TopicPattern: dbs.TopicPattern,
		Description:  dbs.Description.String,
		Tags:         dbs.Tags,
		CreatedAt:    dbs.CreatedAt.Time,
		UpdatedAt:    dbs.UpdatedAt.Time,
	}
}

func dbVersionToVersion(dbv db.SchemaVersion) *SchemaVersion {
	return &SchemaVersion{
		ID:             dbv.ID,
		SchemaID:       dbv.SchemaID,
		Version:        dbv.Version,
		SchemaJSON:     dbv.SchemaJson,
		ValidationMode: ValidationMode(dbv.ValidationMode.String),
		OnInvalid:      OnInvalid(dbv.OnInvalid.String),
		Compatibility:  Compatibility(dbv.Compatibility.String),
		Examples:       dbv.Examples,
		Fingerprint:    dbv.Fingerprint.String,
		IsLatest:       dbv.IsLatest.Bool,
		CreatedAt:      dbv.CreatedAt.Time,
		CreatedBy:      dbv.CreatedBy.String,
	}
}

package display

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/filipexyz/notif/pkg/client"
)

const (
	// CacheTTL is the default cache time-to-live.
	CacheTTL = 5 * time.Minute
	// CacheDir is the subdirectory for schema cache.
	CacheDir = "cache/schemas"
)

// ConfigLoader loads and merges display configurations from multiple sources.
type ConfigLoader struct {
	client    *client.Client
	cacheDir  string
	ttl       time.Duration
	schemas   map[string]*SchemaWithDisplay
	index     *CacheIndex
	mu        sync.RWMutex
	noCache   bool
	offline   bool
}

// NewConfigLoader creates a new config loader.
func NewConfigLoader(c *client.Client) *ConfigLoader {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".notif", CacheDir)

	return &ConfigLoader{
		client:   c,
		cacheDir: cacheDir,
		ttl:      CacheTTL,
		schemas:  make(map[string]*SchemaWithDisplay),
	}
}

// WithNoCache disables cache usage.
func (l *ConfigLoader) WithNoCache() *ConfigLoader {
	l.noCache = true
	return l
}

// WithOffline uses only local cache.
func (l *ConfigLoader) WithOffline() *ConfigLoader {
	l.offline = true
	return l
}

// Load loads schemas with display configs from cache or server.
func (l *ConfigLoader) Load(ctx context.Context) error {
	if l.noCache {
		return l.fetchFromServer(ctx)
	}

	// Try to load from cache
	if l.loadFromDisk() {
		if !l.isExpired() {
			return nil // Cache is valid
		}
		// Cache exists but expired - refresh in background unless offline
		if !l.offline {
			go func() {
				// Ignore errors in background refresh
				_ = l.fetchFromServer(context.Background())
			}()
		}
		return nil // Use stale cache while refreshing
	}

	// No cache, must fetch
	if l.offline {
		return fmt.Errorf("no cache available and offline mode is enabled")
	}

	return l.fetchFromServer(ctx)
}

// loadFromDisk loads the cache from disk.
func (l *ConfigLoader) loadFromDisk() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Load index
	indexPath := filepath.Join(l.cacheDir, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return false
	}

	var index CacheIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return false
	}
	l.index = &index

	// Load schemas
	schemasPath := filepath.Join(l.cacheDir, "schemas.json")
	schemasData, err := os.ReadFile(schemasPath)
	if err != nil {
		return false
	}

	var cache SchemasCache
	if err := json.Unmarshal(schemasData, &cache); err != nil {
		return false
	}

	// Build map
	l.schemas = make(map[string]*SchemaWithDisplay)
	for i := range cache.Schemas {
		s := &cache.Schemas[i]
		l.schemas[s.Name] = s
	}

	return true
}

// isExpired checks if the cache has expired.
func (l *ConfigLoader) isExpired() bool {
	if l.index == nil {
		return true
	}

	lastSync, err := time.Parse(time.RFC3339, l.index.LastSync)
	if err != nil {
		return true
	}

	ttl := l.ttl
	if l.index.TTL > 0 {
		ttl = time.Duration(l.index.TTL) * time.Second
	}

	return time.Since(lastSync) > ttl
}

// fetchFromServer fetches schemas from the server.
func (l *ConfigLoader) fetchFromServer(ctx context.Context) error {
	if l.client == nil {
		return fmt.Errorf("no client available")
	}

	// Fetch schemas with display config
	// Note: This requires the server to support the display extension
	result, err := l.client.SchemaList()
	if err != nil {
		return fmt.Errorf("failed to fetch schemas: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Build schemas map
	l.schemas = make(map[string]*SchemaWithDisplay)
	var cacheSchemas []SchemaWithDisplay

	for _, s := range result.Schemas {
		schema := SchemaWithDisplay{
			Name:         s.Name,
			TopicPattern: s.TopicPattern,
		}

		// Extract display config from schema if available
		if s.LatestVersion != nil && len(s.LatestVersion.Schema) > 0 {
			schema.Schema = s.LatestVersion.Schema
			schema.Display = extractDisplayFromSchema(s.LatestVersion.Schema)
		}

		l.schemas[s.Name] = &schema
		cacheSchemas = append(cacheSchemas, schema)
	}

	// Update index
	l.index = &CacheIndex{
		Server:      l.client.ServerURL(),
		LastSync:    time.Now().Format(time.RFC3339),
		TTL:         int(l.ttl.Seconds()),
		SchemaCount: len(l.schemas),
	}

	// Save to disk (ignore errors)
	l.saveToDisk(cacheSchemas)

	return nil
}

// extractDisplayFromSchema extracts x-notif-display from a JSON Schema.
func extractDisplayFromSchema(schemaJSON json.RawMessage) *DisplayConfig {
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return nil
	}

	displayRaw, ok := schema["x-notif-display"]
	if !ok {
		return nil
	}

	displayJSON, err := json.Marshal(displayRaw)
	if err != nil {
		return nil
	}

	var display DisplayConfig
	if err := json.Unmarshal(displayJSON, &display); err != nil {
		return nil
	}

	return &display
}

// saveToDisk saves the cache to disk.
func (l *ConfigLoader) saveToDisk(schemas []SchemaWithDisplay) {
	// Create directory
	if err := os.MkdirAll(l.cacheDir, 0700); err != nil {
		return
	}

	// Save index
	indexPath := filepath.Join(l.cacheDir, "index.json")
	if indexData, err := json.MarshalIndent(l.index, "", "  "); err == nil {
		_ = os.WriteFile(indexPath, indexData, 0600)
	}

	// Save schemas
	schemasPath := filepath.Join(l.cacheDir, "schemas.json")
	cache := SchemasCache{Schemas: schemas}
	if schemasData, err := json.MarshalIndent(cache, "", "  "); err == nil {
		_ = os.WriteFile(schemasPath, schemasData, 0600)
	}
}

// GetDisplayForTopic finds the display config for a topic.
func (l *ConfigLoader) GetDisplayForTopic(topic string) *DisplayConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Find schema with matching topic pattern
	for _, schema := range l.schemas {
		if schema.Display != nil && matchSchemaTopicPattern(schema.TopicPattern, topic) {
			return schema.Display
		}
	}

	return nil
}

// GetAllSchemas returns all cached schemas.
func (l *ConfigLoader) GetAllSchemas() []*SchemaWithDisplay {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*SchemaWithDisplay, 0, len(l.schemas))
	for _, s := range l.schemas {
		result = append(result, s)
	}
	return result
}

// CacheInfo returns information about the cache.
func (l *ConfigLoader) CacheInfo() *CacheIndex {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.index
}

// ClearCache removes the local cache.
func (l *ConfigLoader) ClearCache() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.schemas = make(map[string]*SchemaWithDisplay)
	l.index = nil

	// Remove cache directory
	if err := os.RemoveAll(l.cacheDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Refresh forces a cache refresh.
func (l *ConfigLoader) Refresh(ctx context.Context) error {
	return l.fetchFromServer(ctx)
}

// matchSchemaTopicPattern checks if a topic matches a schema's topic pattern.
func matchSchemaTopicPattern(pattern, topic string) bool {
	if pattern == "" || topic == "" {
		return false
	}

	// Handle exact match
	if pattern == topic {
		return true
	}

	// Handle wildcards (* matches one segment)
	patternParts := splitTopic(pattern)
	topicParts := splitTopic(topic)

	// Check for ** (match multiple segments)
	hasDoubleWildcard := false
	for _, p := range patternParts {
		if p == "**" {
			hasDoubleWildcard = true
			break
		}
	}

	if hasDoubleWildcard {
		return matchDoubleWildcard(patternParts, topicParts)
	}

	// Single wildcard matching
	if len(patternParts) != len(topicParts) {
		return false
	}

	for i, p := range patternParts {
		if p == "*" {
			continue
		}
		if p != topicParts[i] {
			return false
		}
	}

	return true
}

// matchDoubleWildcard handles ** patterns.
func matchDoubleWildcard(patternParts, topicParts []string) bool {
	pi := 0 // pattern index
	ti := 0 // topic index

	for pi < len(patternParts) && ti < len(topicParts) {
		if patternParts[pi] == "**" {
			// ** at end matches everything remaining
			if pi == len(patternParts)-1 {
				return true
			}
			// Try to match next pattern part
			nextPattern := patternParts[pi+1]
			for ti < len(topicParts) {
				if topicParts[ti] == nextPattern || nextPattern == "*" {
					pi++
					break
				}
				ti++
			}
		} else if patternParts[pi] == "*" || patternParts[pi] == topicParts[ti] {
			pi++
			ti++
		} else {
			return false
		}
	}

	// Check remaining pattern parts (should all be ** or we've matched everything)
	for pi < len(patternParts) {
		if patternParts[pi] != "**" {
			return false
		}
		pi++
	}

	return ti == len(topicParts)
}

// LoadProjectConfig loads display config from .notif.json in the current directory.
func LoadProjectConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(".notif.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config file
		}
		return nil, err
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid .notif.json: %w", err)
	}

	return &cfg, nil
}

// MergeConfigs merges display configs from multiple sources.
// Priority: CLI flags > project config > schema cache > default
func MergeConfigs(cliFormat string, projectCfg *ProjectConfig, schemaCfg *DisplayConfig, topic string) *DisplayConfig {
	// CLI flag takes highest priority
	if cliFormat != "" {
		return &DisplayConfig{Template: cliFormat}
	}

	// Project config for specific topic
	if projectCfg != nil && projectCfg.Display != nil && projectCfg.Display.Topics != nil {
		// Try exact match
		if cfg, ok := projectCfg.Display.Topics[topic]; ok && cfg != nil {
			return cfg
		}
		// Try pattern match
		for pattern, cfg := range projectCfg.Display.Topics {
			if cfg != nil && matchSchemaTopicPattern(pattern, topic) {
				return cfg
			}
		}
	}

	// Schema cache
	if schemaCfg != nil {
		return schemaCfg
	}

	// Default (nil means use default renderer)
	return nil
}

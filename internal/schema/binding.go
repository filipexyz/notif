package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Binding represents a topic-to-schema binding
type Binding struct {
	TopicPattern string `json:"topic_pattern"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Version      string `json:"version"`
}

// BindingStore manages topic bindings
type BindingStore struct {
	configDir string
	bindings  []Binding
}

// NewBindingStore creates a new binding store
func NewBindingStore() (*BindingStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".notif")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	store := &BindingStore{
		configDir: configDir,
	}

	// Load existing bindings
	if err := store.load(); err != nil {
		// If file doesn't exist, start with empty bindings
		store.bindings = []Binding{}
	}

	return store, nil
}

// Add adds a new binding
func (s *BindingStore) Add(topicPattern, namespace, name, version string) error {
	// Validate topic pattern
	if !isValidTopicPattern(topicPattern) {
		return fmt.Errorf("invalid topic pattern: must use * or # as wildcards")
	}

	// Check if binding already exists
	for i, b := range s.bindings {
		if b.TopicPattern == topicPattern {
			// Update existing binding
			s.bindings[i] = Binding{
				TopicPattern: topicPattern,
				Namespace:    namespace,
				Name:         name,
				Version:      version,
			}
			return s.save()
		}
	}

	// Add new binding
	s.bindings = append(s.bindings, Binding{
		TopicPattern: topicPattern,
		Namespace:    namespace,
		Name:         name,
		Version:      version,
	})

	return s.save()
}

// Remove removes a binding by topic pattern
func (s *BindingStore) Remove(topicPattern string) error {
	found := false
	newBindings := []Binding{}

	for _, b := range s.bindings {
		if b.TopicPattern != topicPattern {
			newBindings = append(newBindings, b)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("binding not found: %s", topicPattern)
	}

	s.bindings = newBindings
	return s.save()
}

// List returns all bindings
func (s *BindingStore) List() []Binding {
	return s.bindings
}

// GetForTopic returns the schema binding for a specific topic
func (s *BindingStore) GetForTopic(topic string) *Binding {
	// Check for exact match first
	for _, b := range s.bindings {
		if b.TopicPattern == topic {
			return &b
		}
	}

	// Check for pattern match
	for _, b := range s.bindings {
		if matchTopicPattern(b.TopicPattern, topic) {
			return &b
		}
	}

	return nil
}

// save saves bindings to disk
func (s *BindingStore) save() error {
	path := filepath.Join(s.configDir, "bindings.json")

	data, err := json.MarshalIndent(s.bindings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bindings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write bindings: %w", err)
	}

	return nil
}

// load loads bindings from disk
func (s *BindingStore) load() error {
	path := filepath.Join(s.configDir, "bindings.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &s.bindings); err != nil {
		return fmt.Errorf("failed to unmarshal bindings: %w", err)
	}

	return nil
}

// ValidatePayload validates a payload against the schema bound to a topic
func (s *BindingStore) ValidatePayload(topic string, payload map[string]interface{}) error {
	binding := s.GetForTopic(topic)
	if binding == nil {
		// No binding, no validation
		return nil
	}

	// Load schema
	storage, err := NewStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	schema, _, _, err := storage.Load(binding.Namespace, binding.Name, binding.Version)
	if err != nil {
		return fmt.Errorf("schema not installed: @%s/%s@%s (try: notif schema install @%s/%s@%s)",
			binding.Namespace, binding.Name, binding.Version,
			binding.Namespace, binding.Name, binding.Version)
	}

	// Validate each field
	for fieldName, field := range schema.Fields {
		value, exists := payload[fieldName]

		// Check required fields
		if field.Required && !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}

		// Validate type if field exists
		if exists {
			if err := validateFieldValue(fieldName, *field, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateFieldValue(fieldName string, field Field, value interface{}) error {
	switch field.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s: expected string, got %T", fieldName, value)
		}
	case "integer":
		switch value.(type) {
		case int, int64, float64:
			// Allow numeric types
		default:
			return fmt.Errorf("field %s: expected integer, got %T", fieldName, value)
		}
	case "number":
		switch value.(type) {
		case int, int64, float64:
			// Allow numeric types
		default:
			return fmt.Errorf("field %s: expected number, got %T", fieldName, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %s: expected boolean, got %T", fieldName, value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field %s: expected array, got %T", fieldName, value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field %s: expected object, got %T", fieldName, value)
		}
	}

	return nil
}

// isValidTopicPattern checks if a topic pattern is valid
func isValidTopicPattern(pattern string) bool {
	// Simple validation: allow alphanumeric, dots, *, and #
	match, _ := regexp.MatchString(`^[a-zA-Z0-9.*#_-]+$`, pattern)
	return match
}

// matchTopicPattern checks if a topic matches a pattern
func matchTopicPattern(pattern, topic string) bool {
	// Convert topic pattern to regex
	// * matches one level
	// # matches multiple levels
	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = regexp.MustCompile(`\\\*`).ReplaceAllString(regexPattern, `[^.]+`)
	regexPattern = regexp.MustCompile(`\\\#`).ReplaceAllString(regexPattern, `.+`)
	regexPattern = "^" + regexPattern + "$"

	match, err := regexp.MatchString(regexPattern, topic)
	if err != nil {
		return false
	}

	return match
}

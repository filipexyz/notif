package display

import (
	"encoding/json"
	"fmt"
	"time"
)

// Renderer is the interface for event display renderers.
type Renderer interface {
	Render(event *EventData) (string, error)
}

// DefaultRenderer is the fallback renderer that mimics the original output format.
type DefaultRenderer struct {
	colorizer *Colorizer
}

// NewDefaultRenderer creates a new default renderer.
func NewDefaultRenderer(colorizer *Colorizer) *DefaultRenderer {
	return &DefaultRenderer{colorizer: colorizer}
}

// Render renders an event in the default format: timestamp topic {json}
func (r *DefaultRenderer) Render(event *EventData) (string, error) {
	// Format timestamp
	ts := event.Timestamp
	if ts == "" {
		ts = time.Now().Format("15:04:05")
	} else {
		// Try to parse and reformat
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			ts = t.Format("15:04:05")
		}
	}

	// Get topic string
	var topic string
	switch t := event.Topic.(type) {
	case string:
		topic = t
	case map[string]interface{}:
		if full, ok := t["_full"].(string); ok {
			topic = full
		} else {
			topic = fmt.Sprint(t)
		}
	default:
		topic = fmt.Sprint(t)
	}

	// Format data as JSON
	dataJSON, _ := json.Marshal(event.Data)

	return fmt.Sprintf("%s %s %s",
		r.colorizer.Dim(ts),
		r.colorizer.Color(topic, "magenta"),
		string(dataJSON),
	), nil
}

// NewRenderer creates the appropriate renderer based on the config.
func NewRenderer(cfg *DisplayConfig, colorizer *Colorizer) (Renderer, error) {
	if cfg == nil {
		return NewDefaultRenderer(colorizer), nil
	}

	// If template is specified, use template renderer
	if cfg.Template != "" {
		return NewTemplateRenderer(cfg, colorizer)
	}

	// If fields are specified, use table renderer
	if len(cfg.Fields) > 0 {
		return NewTableRenderer(cfg, colorizer)
	}

	// Fallback to default
	return NewDefaultRenderer(colorizer), nil
}

// RendererManager manages renderers for different topic patterns.
type RendererManager struct {
	renderers    map[string]Renderer
	defaultRend  Renderer
	colorizer    *Colorizer
	topicPattern string // For extracting topic parts
}

// NewRendererManager creates a new renderer manager.
func NewRendererManager(colorizer *Colorizer) *RendererManager {
	return &RendererManager{
		renderers:   make(map[string]Renderer),
		defaultRend: NewDefaultRenderer(colorizer),
		colorizer:   colorizer,
	}
}

// SetDefaultConfig sets the default display config.
func (m *RendererManager) SetDefaultConfig(cfg *DisplayConfig) error {
	if cfg == nil {
		m.defaultRend = NewDefaultRenderer(m.colorizer)
		return nil
	}

	m.topicPattern = cfg.TopicPattern

	r, err := NewRenderer(cfg, m.colorizer)
	if err != nil {
		return err
	}
	m.defaultRend = r
	return nil
}

// AddTopicConfig adds a renderer for a specific topic pattern.
func (m *RendererManager) AddTopicConfig(topicPattern string, cfg *DisplayConfig) error {
	r, err := NewRenderer(cfg, m.colorizer)
	if err != nil {
		return err
	}
	m.renderers[topicPattern] = r
	return nil
}

// GetRenderer returns the appropriate renderer for a topic.
func (m *RendererManager) GetRenderer(topic string) Renderer {
	// Try exact match first
	if r, ok := m.renderers[topic]; ok {
		return r
	}

	// Try pattern matching
	for pattern, r := range m.renderers {
		if matchTopicPattern(pattern, topic) {
			return r
		}
	}

	return m.defaultRend
}

// RenderEvent renders an event using the appropriate renderer.
func (m *RendererManager) RenderEvent(id, topic string, data json.RawMessage, timestamp time.Time) (string, error) {
	// Parse data
	var dataMap map[string]interface{}
	if err := json.Unmarshal(data, &dataMap); err != nil {
		dataMap = map[string]interface{}{"_raw": string(data)}
	}

	// Build topic data with extracted parts if pattern is set
	topicData := BuildTopicData(topic, m.topicPattern)

	event := &EventData{
		ID:        id,
		Topic:     topicData,
		Timestamp: timestamp.Format(time.RFC3339),
		Data:      dataMap,
	}

	renderer := m.GetRenderer(topic)
	return renderer.Render(event)
}

// matchTopicPattern checks if a topic matches a pattern (supports * and **).
func matchTopicPattern(pattern, topic string) bool {
	// Handle exact match
	if pattern == topic {
		return true
	}

	// Handle wildcards
	// * matches one segment
	// ** matches multiple segments (not implemented yet for simplicity)
	if pattern == "*" {
		return true
	}

	// Simple wildcard matching
	patternParts := splitTopic(pattern)
	topicParts := splitTopic(topic)

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

func splitTopic(topic string) []string {
	if topic == "" {
		return nil
	}
	var parts []string
	current := ""
	for _, c := range topic {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}

// JSONRenderer outputs events as JSON (for --json mode).
type JSONRenderer struct{}

// NewJSONRenderer creates a new JSON renderer.
func NewJSONRenderer() *JSONRenderer {
	return &JSONRenderer{}
}

// Render renders an event as compact JSON.
func (r *JSONRenderer) Render(event *EventData) (string, error) {
	out := map[string]interface{}{
		"id":        event.ID,
		"topic":     event.Topic,
		"data":      event.Data,
		"timestamp": event.Timestamp,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

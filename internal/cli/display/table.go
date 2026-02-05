package display

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// TableRenderer renders events as aligned columns (no header).
type TableRenderer struct {
	fields    []FieldConfig
	colorizer *Colorizer
	// Compiled conditions per field
	fieldConditions []*ConditionEvaluator
}

// NewTableRenderer creates a new table renderer.
func NewTableRenderer(cfg *DisplayConfig, colorizer *Colorizer) (*TableRenderer, error) {
	if len(cfg.Fields) == 0 {
		return nil, fmt.Errorf("at least one field is required for table display")
	}

	r := &TableRenderer{
		fields:          cfg.Fields,
		colorizer:       colorizer,
		fieldConditions: make([]*ConditionEvaluator, len(cfg.Fields)),
	}

	// Compile conditions for each field
	for i, field := range cfg.Fields {
		if len(field.Conditions) > 0 {
			eval, err := NewConditionEvaluator(field.Conditions)
			if err != nil {
				return nil, fmt.Errorf("failed to compile conditions for field %q: %w", field.Path, err)
			}
			r.fieldConditions[i] = eval
		}
	}

	return r, nil
}

// Render renders an event as aligned columns.
func (r *TableRenderer) Render(event *EventData) (string, error) {
	var buf bytes.Buffer

	for i, field := range r.fields {
		if i > 0 {
			buf.WriteString(" ")
		}

		value := r.extractValue(event, field.Path)
		formatted := r.formatValue(value, field)

		// Apply field-level conditions
		color := field.Color
		if r.fieldConditions[i] != nil {
			if cfg := r.fieldConditions[i].EvaluateFirst(value); cfg != nil && cfg.Color != "" {
				color = cfg.Color
			}
		}

		// Apply width/padding
		if field.Width > 0 {
			formatted = r.padToWidth(formatted, field.Width)
		}

		// Apply color
		if color != "" {
			formatted = r.colorizer.Color(formatted, color)
		}

		buf.WriteString(formatted)
	}

	return buf.String(), nil
}

// extractValue extracts a value from event data using a dot-separated path.
func (r *TableRenderer) extractValue(event *EventData, path string) interface{} {
	parts := strings.Split(path, ".")

	// Handle special root paths
	var current interface{}
	switch parts[0] {
	case "id":
		return event.ID
	case "timestamp":
		return event.Timestamp
	case "topic":
		if len(parts) == 1 {
			return event.Topic
		}
		// Handle topic parts if topic is a map
		if m, ok := event.Topic.(map[string]interface{}); ok {
			current = m
			parts = parts[1:]
		} else {
			return event.Topic
		}
	case "data":
		if len(parts) == 1 {
			return event.Data
		}
		current = event.Data
		parts = parts[1:]
	default:
		// Try from data directly
		current = event.Data
	}

	// Navigate nested path
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case map[interface{}]interface{}:
			current = v[part]
		default:
			return nil
		}
		if current == nil {
			return nil
		}
	}

	return current
}

// formatValue formats a value using the field's format string.
func (r *TableRenderer) formatValue(value interface{}, field FieldConfig) string {
	if value == nil {
		return ""
	}

	if field.Format != "" {
		return fmt.Sprintf(field.Format, value)
	}

	return fmt.Sprint(value)
}

// padToWidth pads or truncates a string to exactly the given width.
func (r *TableRenderer) padToWidth(s string, width int) string {
	// Count runes for proper unicode handling
	runeCount := utf8.RuneCountInString(s)

	if runeCount > width {
		// Truncate with ellipsis
		runes := []rune(s)
		if width > 3 {
			return string(runes[:width-3]) + "..."
		}
		return string(runes[:width])
	}

	if runeCount < width {
		// Pad with spaces
		return s + strings.Repeat(" ", width-runeCount)
	}

	return s
}

// Fields returns the configured fields.
func (r *TableRenderer) Fields() []FieldConfig {
	return r.fields
}

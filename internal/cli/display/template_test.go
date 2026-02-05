package display

import (
	"testing"
)

func TestExtractTopicParts(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		topic   string
		want    map[string]string
	}{
		{
			name:    "simple pattern",
			pattern: "orders.{action}",
			topic:   "orders.created",
			want:    map[string]string{"action": "created"},
		},
		{
			name:    "two placeholders",
			pattern: "orders.{action}.{id}",
			topic:   "orders.paid.123",
			want:    map[string]string{"action": "paid", "id": "123"},
		},
		{
			name:    "middle placeholder",
			pattern: "users.{userId}.events",
			topic:   "users.abc123.events",
			want:    map[string]string{"userId": "abc123"},
		},
		{
			name:    "multiple segments",
			pattern: "{service}.logs.{level}",
			topic:   "api.logs.error",
			want:    map[string]string{"service": "api", "level": "error"},
		},
		{
			name:    "no match - different structure",
			pattern: "orders.{action}",
			topic:   "users.created",
			want:    nil,
		},
		{
			name:    "no match - wrong segment count",
			pattern: "orders.{action}.{id}",
			topic:   "orders.created",
			want:    nil,
		},
		{
			name:    "empty pattern",
			pattern: "",
			topic:   "orders.created",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTopicParts(tt.pattern, tt.topic)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ExtractTopicParts() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("ExtractTopicParts() = nil, want %v", tt.want)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ExtractTopicParts()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestTemplateRenderer_BasicTemplate(t *testing.T) {
	colorizer := NewColorizer(false) // disable colors for testing
	cfg := &DisplayConfig{
		Template: "Order {{.data.orderId}} - Status: {{.data.status}}",
	}

	renderer, err := NewTemplateRenderer(cfg, colorizer)
	if err != nil {
		t.Fatalf("NewTemplateRenderer() error = %v", err)
	}

	event := &EventData{
		ID:        "evt_123",
		Topic:     "orders.created",
		Timestamp: "2025-02-05T15:04:05Z",
		Data: map[string]interface{}{
			"orderId": "ORD-001",
			"status":  "pending",
		},
	}

	got, err := renderer.Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "Order ORD-001 - Status: pending"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestTemplateRenderer_WithFunctions(t *testing.T) {
	colorizer := NewColorizer(false)
	cfg := &DisplayConfig{
		Template: "{{.data.name | upper}} - {{.data.amount | printf \"$%.2f\"}}",
	}

	renderer, err := NewTemplateRenderer(cfg, colorizer)
	if err != nil {
		t.Fatalf("NewTemplateRenderer() error = %v", err)
	}

	event := &EventData{
		ID:    "evt_123",
		Topic: "orders.created",
		Data: map[string]interface{}{
			"name":   "test",
			"amount": 99.5,
		},
	}

	got, err := renderer.Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "TEST - $99.50"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestTemplateRenderer_TopicPattern(t *testing.T) {
	colorizer := NewColorizer(false)
	cfg := &DisplayConfig{
		TopicPattern: "orders.{action}.{id}",
		Template:     "[{{.topic.action}}] Order #{{.topic.id}}",
	}

	renderer, err := NewTemplateRenderer(cfg, colorizer)
	if err != nil {
		t.Fatalf("NewTemplateRenderer() error = %v", err)
	}

	// Build topic data with extracted parts
	topicData := BuildTopicData("orders.paid.123", cfg.TopicPattern)

	event := &EventData{
		ID:    "evt_123",
		Topic: topicData,
		Data:  map[string]interface{}{},
	}

	got, err := renderer.Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "[paid] Order #123"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestTemplateRenderer_Conditionals(t *testing.T) {
	colorizer := NewColorizer(false)
	cfg := &DisplayConfig{
		Template: `{{if eq .data.status "paid"}}✅{{else}}⏳{{end}} Order {{.data.orderId}}`,
	}

	renderer, err := NewTemplateRenderer(cfg, colorizer)
	if err != nil {
		t.Fatalf("NewTemplateRenderer() error = %v", err)
	}

	tests := []struct {
		status string
		want   string
	}{
		{"paid", "✅ Order ORD-001"},
		{"pending", "⏳ Order ORD-001"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			event := &EventData{
				ID:    "evt_123",
				Topic: "orders.updated",
				Data: map[string]interface{}{
					"orderId": "ORD-001",
					"status":  tt.status,
				},
			}

			got, err := renderer.Render(event)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTopicData(t *testing.T) {
	tests := []struct {
		name         string
		topic        string
		topicPattern string
		wantString   bool // true if result should be string, false if map
	}{
		{
			name:         "no pattern",
			topic:        "orders.created",
			topicPattern: "",
			wantString:   true,
		},
		{
			name:         "with pattern",
			topic:        "orders.created.123",
			topicPattern: "orders.{action}.{id}",
			wantString:   false,
		},
		{
			name:         "pattern no match",
			topic:        "users.created",
			topicPattern: "orders.{action}",
			wantString:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTopicData(tt.topic, tt.topicPattern)
			if tt.wantString {
				if _, ok := got.(string); !ok {
					t.Errorf("BuildTopicData() = %T, want string", got)
				}
			} else {
				if _, ok := got.(map[string]interface{}); !ok {
					t.Errorf("BuildTopicData() = %T, want map", got)
				}
			}
		})
	}
}

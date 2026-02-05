package display

import (
	"testing"
)

func TestCompileCondition(t *testing.T) {
	tests := []struct {
		name    string
		when    string
		wantErr bool
	}{
		{
			name:    "simple equality",
			when:    `.status == "error"`,
			wantErr: false,
		},
		{
			name:    "numeric comparison",
			when:    `.amount > 100`,
			wantErr: false,
		},
		{
			name:    "nested field",
			when:    `.data.customer.name == "John"`,
			wantErr: false,
		},
		{
			name:    "invalid jq",
			when:    `this is not valid`,
			wantErr: true,
		},
		{
			name:    "empty when",
			when:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConditionConfig{When: tt.when}
			_, err := CompileCondition(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompileCondition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompiledCondition_Evaluate(t *testing.T) {
	tests := []struct {
		name string
		when string
		data interface{}
		want bool
	}{
		{
			name: "equality match",
			when: `.status == "error"`,
			data: map[string]interface{}{"status": "error"},
			want: true,
		},
		{
			name: "equality no match",
			when: `.status == "error"`,
			data: map[string]interface{}{"status": "success"},
			want: false,
		},
		{
			name: "numeric greater than - match",
			when: `.amount > 100`,
			data: map[string]interface{}{"amount": 150.0},
			want: true,
		},
		{
			name: "numeric greater than - no match",
			when: `.amount > 100`,
			data: map[string]interface{}{"amount": 50.0},
			want: false,
		},
		{
			name: "nested field",
			when: `.customer.tier == "premium"`,
			data: map[string]interface{}{
				"customer": map[string]interface{}{
					"tier": "premium",
				},
			},
			want: true,
		},
		{
			name: "missing field",
			when: `.notexist == "value"`,
			data: map[string]interface{}{"other": "field"},
			want: false,
		},
		{
			name: "current value check (for field conditions)",
			when: `. == "paid"`,
			data: "paid",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConditionConfig{When: tt.when}
			compiled, err := CompileCondition(cfg)
			if err != nil {
				t.Fatalf("CompileCondition() error = %v", err)
			}

			got := compiled.Evaluate(tt.data)
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionEvaluator_EvaluateFirst(t *testing.T) {
	configs := []ConditionConfig{
		{When: `.status == "error"`, Color: "red"},
		{When: `.status == "success"`, Color: "green"},
		{When: `.status == "pending"`, Color: "yellow"},
	}

	eval, err := NewConditionEvaluator(configs)
	if err != nil {
		t.Fatalf("NewConditionEvaluator() error = %v", err)
	}

	tests := []struct {
		status    string
		wantColor string
		wantNil   bool
	}{
		{"error", "red", false},
		{"success", "green", false},
		{"pending", "yellow", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			data := map[string]interface{}{"status": tt.status}
			got := eval.EvaluateFirst(data)

			if tt.wantNil {
				if got != nil {
					t.Errorf("EvaluateFirst() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("EvaluateFirst() = nil, want config with color %q", tt.wantColor)
				return
			}

			if got.Color != tt.wantColor {
				t.Errorf("EvaluateFirst().Color = %q, want %q", got.Color, tt.wantColor)
			}
		})
	}
}

func TestConditionEvaluator_ApplyConditions(t *testing.T) {
	configs := []ConditionConfig{
		{
			When:   `.status == "error"`,
			Color:  "red",
			Prefix: "❌ ",
			Set:    map[string]string{"icon": "error_icon"},
		},
		{
			When:   `.status == "success"`,
			Color:  "green",
			Prefix: "✅ ",
		},
	}

	eval, err := NewConditionEvaluator(configs)
	if err != nil {
		t.Fatalf("NewConditionEvaluator() error = %v", err)
	}

	data := map[string]interface{}{"status": "error"}
	color, prefix, suffix, vars := eval.ApplyConditions(data)

	if color != "red" {
		t.Errorf("color = %q, want %q", color, "red")
	}
	if prefix != "❌ " {
		t.Errorf("prefix = %q, want %q", prefix, "❌ ")
	}
	if suffix != "" {
		t.Errorf("suffix = %q, want empty", suffix)
	}
	if vars["icon"] != "error_icon" {
		t.Errorf("vars[icon] = %q, want %q", vars["icon"], "error_icon")
	}
}

func TestEvaluateConditionForValue(t *testing.T) {
	conditions := []ConditionConfig{
		{When: `. == "paid"`, Color: "green"},
		{When: `. == "pending"`, Color: "yellow"},
		{When: `. == "failed"`, Color: "red"},
	}

	tests := []struct {
		value     interface{}
		wantColor string
		wantNil   bool
	}{
		{"paid", "green", false},
		{"pending", "yellow", false},
		{"failed", "red", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.value.(string), func(t *testing.T) {
			got := EvaluateConditionForValue(conditions, tt.value)

			if tt.wantNil {
				if got != nil {
					t.Errorf("EvaluateConditionForValue() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Errorf("EvaluateConditionForValue() = nil, want config")
				return
			}

			if got.Color != tt.wantColor {
				t.Errorf("Color = %q, want %q", got.Color, tt.wantColor)
			}
		})
	}
}

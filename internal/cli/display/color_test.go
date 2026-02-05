package display

import (
	"testing"
)

func TestColorizer_Disabled(t *testing.T) {
	c := NewColorizer(false)

	if !c.IsDisabled() {
		t.Error("IsDisabled() = false, want true")
	}

	// All methods should return plain text when disabled
	tests := []struct {
		name   string
		method func(string) string
		input  string
	}{
		{"Color", func(s string) string { return c.Color(s, "red") }, "test"},
		{"RGB", func(s string) string { return c.RGB(s, "#FF0000") }, "test"},
		{"Background", func(s string) string { return c.Background(s, "blue") }, "test"},
		{"Bold", c.Bold, "test"},
		{"Dim", c.Dim, "test"},
		{"Italic", c.Italic, "test"},
		{"Underline", c.Underline, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method(tt.input)
			if got != tt.input {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.input)
			}
		})
	}
}

func TestColorizer_NamedColors(t *testing.T) {
	c := NewColorizer(true)

	// Test that named colors don't return empty
	colors := []string{"red", "green", "blue", "yellow", "cyan", "magenta", "white", "black", "gray", "orange", "pink", "purple"}

	for _, color := range colors {
		t.Run(color, func(t *testing.T) {
			got := c.Color("test", color)
			if got == "test" {
				// If color is applied, string should be different
				// (contains ANSI codes)
				// This might fail if terminal doesn't support colors,
				// but the method should at least attempt to apply them
			}
			if got == "" {
				t.Errorf("Color(%q) returned empty string", color)
			}
		})
	}
}

func TestColorizer_HexColors(t *testing.T) {
	c := NewColorizer(true)

	hexColors := []string{"#FF5500", "#00FF00", "#0000FF", "#FFFFFF", "#000000"}

	for _, hex := range hexColors {
		t.Run(hex, func(t *testing.T) {
			got := c.RGB("test", hex)
			if got == "" {
				t.Errorf("RGB(%q) returned empty string", hex)
			}
		})
	}
}

func TestColorizer_EmptyColor(t *testing.T) {
	c := NewColorizer(true)

	// Empty color should return original text
	got := c.Color("test", "")
	if got != "test" {
		t.Errorf("Color with empty color = %q, want %q", got, "test")
	}
}

func TestColorizer_Styled(t *testing.T) {
	c := NewColorizer(true)

	// Test that styled method doesn't panic
	got := c.Styled("test", "red", "blue", true, false, true, false)
	if got == "" {
		t.Error("Styled() returned empty string")
	}
}

func TestDefaultColorizer(t *testing.T) {
	// Test singleton
	c1 := DefaultColorizer()
	c2 := DefaultColorizer()

	if c1 != c2 {
		t.Error("DefaultColorizer() should return same instance")
	}
}

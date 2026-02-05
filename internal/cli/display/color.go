package display

import (
	"os"
	"strings"
	"sync"

	"github.com/muesli/termenv"
)

// Colorizer handles terminal color output with RGB support.
type Colorizer struct {
	profile  termenv.Profile
	disabled bool
}

var (
	defaultColorizer *Colorizer
	colorizerOnce    sync.Once
)

// DefaultColorizer returns the singleton colorizer instance.
func DefaultColorizer() *Colorizer {
	colorizerOnce.Do(func() {
		noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
		defaultColorizer = NewColorizer(!noColor)
	})
	return defaultColorizer
}

// NewColorizer creates a new colorizer instance.
func NewColorizer(enabled bool) *Colorizer {
	c := &Colorizer{
		profile:  termenv.ColorProfile(),
		disabled: !enabled,
	}
	return c
}

// namedColors maps color names to RGB hex values.
var namedColors = map[string]string{
	"black":   "#000000",
	"red":     "#FF5555",
	"green":   "#50FA7B",
	"yellow":  "#F1FA8C",
	"blue":    "#6272A4",
	"magenta": "#FF79C6",
	"cyan":    "#8BE9FD",
	"white":   "#F8F8F2",
	"gray":    "#6272A4",
	"grey":    "#6272A4",
	"orange":  "#FFB86C",
	"pink":    "#FF79C6",
	"purple":  "#BD93F9",
}

// resolveColor converts a color name or hex code to a termenv.Color.
func (c *Colorizer) resolveColor(color string) termenv.Color {
	if color == "" {
		return nil
	}

	// Check named colors first
	color = strings.ToLower(color)
	if hex, ok := namedColors[color]; ok {
		return c.profile.Color(hex)
	}

	// Handle hex colors
	if strings.HasPrefix(color, "#") {
		return c.profile.Color(color)
	}

	// Try as-is (for ANSI color names)
	return c.profile.Color(color)
}

// Color applies a foreground color to text.
func (c *Colorizer) Color(text, color string) string {
	if c.disabled || color == "" {
		return text
	}
	col := c.resolveColor(color)
	if col == nil {
		return text
	}
	return termenv.String(text).Foreground(col).String()
}

// RGB applies an RGB foreground color to text.
func (c *Colorizer) RGB(text, hex string) string {
	return c.Color(text, hex)
}

// Background applies a background color to text.
func (c *Colorizer) Background(text, color string) string {
	if c.disabled || color == "" {
		return text
	}
	col := c.resolveColor(color)
	if col == nil {
		return text
	}
	return termenv.String(text).Background(col).String()
}

// Bold makes text bold.
func (c *Colorizer) Bold(text string) string {
	if c.disabled {
		return text
	}
	return termenv.String(text).Bold().String()
}

// Dim makes text dimmed.
func (c *Colorizer) Dim(text string) string {
	if c.disabled {
		return text
	}
	return termenv.String(text).Faint().String()
}

// Italic makes text italic.
func (c *Colorizer) Italic(text string) string {
	if c.disabled {
		return text
	}
	return termenv.String(text).Italic().String()
}

// Underline underlines text.
func (c *Colorizer) Underline(text string) string {
	if c.disabled {
		return text
	}
	return termenv.String(text).Underline().String()
}

// Strikethrough strikes through text.
func (c *Colorizer) Strikethrough(text string) string {
	if c.disabled {
		return text
	}
	return termenv.String(text).CrossOut().String()
}

// Reset returns the reset sequence.
func (c *Colorizer) Reset() string {
	if c.disabled {
		return ""
	}
	return termenv.CSI + termenv.ResetSeq + "m"
}

// Styled applies multiple styles to text.
func (c *Colorizer) Styled(text string, fg, bg string, bold, dim, italic, underline bool) string {
	if c.disabled {
		return text
	}

	s := termenv.String(text)

	if fg != "" {
		if col := c.resolveColor(fg); col != nil {
			s = s.Foreground(col)
		}
	}
	if bg != "" {
		if col := c.resolveColor(bg); col != nil {
			s = s.Background(col)
		}
	}
	if bold {
		s = s.Bold()
	}
	if dim {
		s = s.Faint()
	}
	if italic {
		s = s.Italic()
	}
	if underline {
		s = s.Underline()
	}

	return s.String()
}

// IsDisabled returns whether colors are disabled.
func (c *Colorizer) IsDisabled() bool {
	return c.disabled
}

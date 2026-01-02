package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
	Bold    = "\033[1m"
)

// Output handles CLI output formatting.
type Output struct {
	jsonMode bool
	noColor  bool
}

// New creates a new Output instance.
func New(jsonMode bool) *Output {
	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	return &Output{jsonMode: jsonMode, noColor: noColor}
}

func (o *Output) color(c, text string) string {
	if o.noColor {
		return text
	}
	return c + text + Reset
}

// Success prints a success message.
func (o *Output) Success(format string, args ...any) {
	if o.jsonMode {
		return
	}
	fmt.Printf(o.color(Green, "✓ ")+format+"\n", args...)
}

// Error prints an error message.
func (o *Output) Error(format string, args ...any) {
	if o.jsonMode {
		return
	}
	fmt.Fprintf(os.Stderr, o.color(Red, "✗ ")+format+"\n", args...)
}

// Warn prints a warning message.
func (o *Output) Warn(format string, args ...any) {
	if o.jsonMode {
		return
	}
	fmt.Printf(o.color(Yellow, "! ")+format+"\n", args...)
}

// Info prints an info message.
func (o *Output) Info(format string, args ...any) {
	if o.jsonMode {
		return
	}
	fmt.Printf(o.color(Cyan, "→ ")+format+"\n", args...)
}

// Header prints a header.
func (o *Output) Header(text string) {
	if o.jsonMode {
		return
	}
	fmt.Println(o.color(Bold, text))
}

// KeyValue prints a key-value pair.
func (o *Output) KeyValue(key, value string) {
	if o.jsonMode {
		return
	}
	fmt.Printf("  %s: %s\n", o.color(Gray, key), value)
}

// Divider prints a divider line.
func (o *Output) Divider() {
	if o.jsonMode {
		return
	}
	fmt.Println(o.color(Gray, "─────────────────────────────────────────"))
}

// JSON prints data as JSON.
func (o *Output) JSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

// Event prints a formatted event.
func (o *Output) Event(id, topic string, data json.RawMessage, ts time.Time) {
	if o.jsonMode {
		// Compact JSON for streaming (one line per event)
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(map[string]any{
			"id":        id,
			"topic":     topic,
			"data":      json.RawMessage(data),
			"timestamp": ts,
		})
		return
	}
	fmt.Printf("%s %s %s\n",
		o.color(Gray, ts.Format("15:04:05")),
		o.color(Magenta, topic),
		string(data),
	)
}

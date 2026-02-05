package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

// TemplateRenderer renders events using Go text/template.
type TemplateRenderer struct {
	tmpl       *template.Template
	colorizer  *Colorizer
	conditions *ConditionEvaluator
	config     *DisplayConfig
}

// NewTemplateRenderer creates a new template renderer.
func NewTemplateRenderer(cfg *DisplayConfig, colorizer *Colorizer) (*TemplateRenderer, error) {
	if cfg.Template == "" {
		return nil, fmt.Errorf("template is required")
	}

	r := &TemplateRenderer{
		colorizer: colorizer,
		config:    cfg,
	}

	// Compile conditions
	if len(cfg.Conditions) > 0 {
		eval, err := NewConditionEvaluator(cfg.Conditions)
		if err != nil {
			return nil, fmt.Errorf("failed to compile conditions: %w", err)
		}
		r.conditions = eval
	}

	// Parse template with custom functions
	funcMap := r.createFuncMap()
	tmpl, err := template.New("event").Funcs(funcMap).Parse(cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	r.tmpl = tmpl

	return r, nil
}

// createFuncMap returns the template function map.
func (r *TemplateRenderer) createFuncMap() template.FuncMap {
	return template.FuncMap{
		// Formatting
		"json":     jsonFormat,
		"time":     timeFormat,
		"printf":   fmt.Sprintf,
		"truncate": truncateString,
		"pad":      padString,
		"padLeft":  padLeft,
		"padRight": padRight,

		// String manipulation
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"title":   strings.Title, //nolint:staticcheck
		"replace": strings.Replace,
		"trim":    strings.TrimSpace,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"split":   strings.Split,
		"join":    strings.Join,

		// Colors and styling
		"color":     r.colorFunc,
		"rgb":       r.rgbFunc,
		"bg":        r.bgFunc,
		"bold":      r.boldFunc,
		"dim":       r.dimFunc,
		"italic":    r.italicFunc,
		"underline": r.underlineFunc,

		// Comparison
		"eq": reflect.DeepEqual,
		"ne": func(a, b interface{}) bool { return !reflect.DeepEqual(a, b) },
		"lt": lessThan,
		"gt": greaterThan,
		"le": lessOrEqual,
		"ge": greaterOrEqual,
		"contains": func(s, substr string) bool { return strings.Contains(s, substr) },
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,

		// Helpers
		"get":      getNestedValue,
		"default":  defaultValue,
		"coalesce": coalesce,
		"len":      length,
		"keys":     keys,
		"values":   values,
		"first":    first,
		"last":     last,
		"index":    indexFunc,
	}
}

// Color functions that use the colorizer
func (r *TemplateRenderer) colorFunc(color string, text interface{}) string {
	return r.colorizer.Color(fmt.Sprint(text), color)
}

func (r *TemplateRenderer) rgbFunc(hex string, text interface{}) string {
	return r.colorizer.RGB(fmt.Sprint(text), hex)
}

func (r *TemplateRenderer) bgFunc(color string, text interface{}) string {
	return r.colorizer.Background(fmt.Sprint(text), color)
}

func (r *TemplateRenderer) boldFunc(text interface{}) string {
	return r.colorizer.Bold(fmt.Sprint(text))
}

func (r *TemplateRenderer) dimFunc(text interface{}) string {
	return r.colorizer.Dim(fmt.Sprint(text))
}

func (r *TemplateRenderer) italicFunc(text interface{}) string {
	return r.colorizer.Italic(fmt.Sprint(text))
}

func (r *TemplateRenderer) underlineFunc(text interface{}) string {
	return r.colorizer.Underline(fmt.Sprint(text))
}

// Render renders an event using the template.
func (r *TemplateRenderer) Render(event *EventData) (string, error) {
	// Build template data
	data := r.buildTemplateData(event)

	// Apply conditions to set additional variables
	if r.conditions != nil {
		color, prefix, suffix, vars := r.conditions.ApplyConditions(data)
		if vars != nil {
			for k, v := range vars {
				data[k] = v
			}
		}
		if prefix != "" {
			data["prefix"] = prefix
		}
		if suffix != "" {
			data["suffix"] = suffix
		}
		data["_conditionColor"] = color
	}

	// Execute template
	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	result := buf.String()

	// Apply condition color to entire output if set
	if color, ok := data["_conditionColor"].(string); ok && color != "" {
		result = r.colorizer.Color(result, color)
	}

	return result, nil
}

// buildTemplateData constructs the data map for template execution.
func (r *TemplateRenderer) buildTemplateData(event *EventData) map[string]interface{} {
	data := make(map[string]interface{})

	data["id"] = event.ID
	data["timestamp"] = event.Timestamp
	data["data"] = event.Data

	// Handle topic - can be string or map with extracted parts
	switch t := event.Topic.(type) {
	case string:
		data["topic"] = t
	case map[string]interface{}:
		data["topic"] = t
	default:
		data["topic"] = fmt.Sprint(event.Topic)
	}

	return data
}

// Helper functions for templates

func jsonFormat(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func timeFormat(layout string, t interface{}) string {
	switch v := t.(type) {
	case time.Time:
		return v.Format(layout)
	case string:
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return v
		}
		return parsed.Format(layout)
	default:
		return fmt.Sprint(v)
	}
}

func truncateString(n int, s string) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	if n > 3 {
		return string(runes[:n-3]) + "..."
	}
	return string(runes[:n])
}

func padString(n int, s string) string {
	return padRight(n, s)
}

func padLeft(n int, s string) string {
	l := utf8.RuneCountInString(s)
	if l >= n {
		return s
	}
	return strings.Repeat(" ", n-l) + s
}

func padRight(n int, s string) string {
	l := utf8.RuneCountInString(s)
	if l >= n {
		return s
	}
	return s + strings.Repeat(" ", n-l)
}

func lessThan(a, b interface{}) bool {
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if aok && bok {
		return af < bf
	}
	return fmt.Sprint(a) < fmt.Sprint(b)
}

func greaterThan(a, b interface{}) bool {
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if aok && bok {
		return af > bf
	}
	return fmt.Sprint(a) > fmt.Sprint(b)
}

func lessOrEqual(a, b interface{}) bool {
	return !greaterThan(a, b)
}

func greaterOrEqual(a, b interface{}) bool {
	return !lessThan(a, b)
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func getNestedValue(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

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

func defaultValue(def, val interface{}) interface{} {
	if val == nil || val == "" {
		return def
	}
	return val
}

func coalesce(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil && v != "" {
			return v
		}
	}
	if len(vals) > 0 {
		return vals[len(vals)-1]
	}
	return nil
}

func length(v interface{}) int {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len()
	default:
		return 0
	}
}

func keys(m interface{}) []string {
	rv := reflect.ValueOf(m)
	if rv.Kind() != reflect.Map {
		return nil
	}
	result := make([]string, 0, rv.Len())
	for _, key := range rv.MapKeys() {
		result = append(result, fmt.Sprint(key.Interface()))
	}
	return result
}

func values(m interface{}) []interface{} {
	rv := reflect.ValueOf(m)
	if rv.Kind() != reflect.Map {
		return nil
	}
	result := make([]interface{}, 0, rv.Len())
	for _, key := range rv.MapKeys() {
		result = append(result, rv.MapIndex(key).Interface())
	}
	return result
}

func first(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if rv.Len() > 0 {
			return rv.Index(0).Interface()
		}
	case reflect.String:
		s := rv.String()
		if len(s) > 0 {
			return string([]rune(s)[0])
		}
	}
	return nil
}

func last(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if rv.Len() > 0 {
			return rv.Index(rv.Len() - 1).Interface()
		}
	case reflect.String:
		s := rv.String()
		runes := []rune(s)
		if len(runes) > 0 {
			return string(runes[len(runes)-1])
		}
	}
	return nil
}

func indexFunc(idx int, v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		if idx >= 0 && idx < rv.Len() {
			return rv.Index(idx).Interface()
		}
	case reflect.String:
		runes := []rune(rv.String())
		if idx >= 0 && idx < len(runes) {
			return string(runes[idx])
		}
	}
	return nil
}

// TopicPattern extraction

// topicPatternRegex matches placeholders like {name} in topic patterns.
var topicPatternRegex = regexp.MustCompile(`\{([^}]+)\}`)

// ExtractTopicParts extracts named parts from a topic using a pattern.
// Pattern: "orders.{action}.{id}" + Topic: "orders.created.123"
// Returns: {"action": "created", "id": "123"}
func ExtractTopicParts(pattern, topic string) map[string]string {
	if pattern == "" {
		return nil
	}

	// Convert pattern to regex
	// "orders.{action}.{id}" -> "^orders\.([^.]+)\.([^.]+)$"
	var names []string
	regexStr := "^"
	lastEnd := 0

	for _, match := range topicPatternRegex.FindAllStringSubmatchIndex(pattern, -1) {
		// Add literal part before this match
		regexStr += regexp.QuoteMeta(pattern[lastEnd:match[0]])
		// Add capture group
		regexStr += `([^.]+)`
		// Extract the name
		names = append(names, pattern[match[2]:match[3]])
		lastEnd = match[1]
	}
	// Add remaining literal part
	regexStr += regexp.QuoteMeta(pattern[lastEnd:])
	regexStr += "$"

	// Compile and match
	re, err := regexp.Compile(regexStr)
	if err != nil {
		return nil
	}

	matches := re.FindStringSubmatch(topic)
	if matches == nil {
		return nil
	}

	// Build result map
	result := make(map[string]string)
	for i, name := range names {
		if i+1 < len(matches) {
			result[name] = matches[i+1]
		}
	}

	return result
}

// BuildTopicData creates the topic data for templates.
// If topicPattern is set, extracts parts and returns a map with both
// the full topic string and extracted parts.
func BuildTopicData(topic, topicPattern string) interface{} {
	if topicPattern == "" {
		return topic
	}

	parts := ExtractTopicParts(topicPattern, topic)
	if parts == nil {
		return topic
	}

	// Create a map with the extracted parts
	// Template can access via {{.topic.action}} etc.
	result := make(map[string]interface{})
	for k, v := range parts {
		result[k] = v
	}
	// Also include the full topic string
	result["_full"] = topic

	return result
}

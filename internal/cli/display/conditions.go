package display

import (
	"encoding/json"
	"fmt"

	"github.com/itchyny/gojq"
)

// CompiledCondition is a pre-compiled jq condition for evaluation.
type CompiledCondition struct {
	code   *gojq.Code
	config ConditionConfig
}

// CompileCondition compiles a condition config into an evaluable form.
func CompileCondition(cfg ConditionConfig) (*CompiledCondition, error) {
	if cfg.When == "" {
		return nil, fmt.Errorf("condition 'when' clause is required")
	}

	query, err := gojq.Parse(cfg.When)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", cfg.When, err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression %q: %w", cfg.When, err)
	}

	return &CompiledCondition{
		code:   code,
		config: cfg,
	}, nil
}

// Evaluate checks if the condition matches the given data.
func (c *CompiledCondition) Evaluate(data interface{}) bool {
	if c.code == nil {
		return false
	}

	iter := c.code.Run(data)
	v, ok := iter.Next()
	if !ok {
		return false
	}

	if _, isErr := v.(error); isErr {
		return false
	}

	if b, ok := v.(bool); ok {
		return b
	}

	return v != nil
}

// Config returns the original condition config.
func (c *CompiledCondition) Config() ConditionConfig {
	return c.config
}

// ConditionEvaluator manages a set of compiled conditions.
type ConditionEvaluator struct {
	conditions []*CompiledCondition
}

// NewConditionEvaluator creates an evaluator from condition configs.
func NewConditionEvaluator(configs []ConditionConfig) (*ConditionEvaluator, error) {
	eval := &ConditionEvaluator{
		conditions: make([]*CompiledCondition, 0, len(configs)),
	}

	for _, cfg := range configs {
		compiled, err := CompileCondition(cfg)
		if err != nil {
			return nil, err
		}
		eval.conditions = append(eval.conditions, compiled)
	}

	return eval, nil
}

// EvaluateFirst returns the first matching condition's config, or nil if none match.
func (e *ConditionEvaluator) EvaluateFirst(data interface{}) *ConditionConfig {
	for _, cond := range e.conditions {
		if cond.Evaluate(data) {
			cfg := cond.Config()
			return &cfg
		}
	}
	return nil
}

// EvaluateAll returns all matching conditions.
func (e *ConditionEvaluator) EvaluateAll(data interface{}) []ConditionConfig {
	var matches []ConditionConfig
	for _, cond := range e.conditions {
		if cond.Evaluate(data) {
			matches = append(matches, cond.Config())
		}
	}
	return matches
}

// ApplyConditions applies the first matching condition to modify the style context.
// Returns the color and any 'set' variables from the matching condition.
func (e *ConditionEvaluator) ApplyConditions(data interface{}) (color string, prefix string, suffix string, vars map[string]string) {
	if cfg := e.EvaluateFirst(data); cfg != nil {
		return cfg.Color, cfg.Prefix, cfg.Suffix, cfg.Set
	}
	return "", "", "", nil
}

// EvaluateConditionForValue evaluates conditions against a single field value.
// Used for per-field conditional formatting.
func EvaluateConditionForValue(conditions []ConditionConfig, value interface{}) *ConditionConfig {
	for _, cfg := range conditions {
		query, err := gojq.Parse(cfg.When)
		if err != nil {
			continue
		}

		code, err := gojq.Compile(query)
		if err != nil {
			continue
		}

		iter := code.Run(value)
		v, ok := iter.Next()
		if !ok {
			continue
		}

		if _, isErr := v.(error); isErr {
			continue
		}

		if b, ok := v.(bool); ok && b {
			return &cfg
		} else if v != nil && !ok {
			return &cfg
		}
	}
	return nil
}

// ParseEventData parses event JSON into a map for condition evaluation.
func ParseEventData(eventJSON json.RawMessage) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(eventJSON, &data); err != nil {
		return nil, err
	}
	return data, nil
}

package cmd

import (
	"encoding/json"

	"github.com/itchyny/gojq"
)

// compileJqFilter parses and compiles a jq filter expression.
func compileJqFilter(filter string) (*gojq.Code, error) {
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}
	return gojq.Compile(query)
}

// matchesJqFilter evaluates a compiled jq filter against JSON data.
// Returns true if the filter matches (expression evaluates to true or non-nil).
// If code is nil, returns true (no filter = match all).
func matchesJqFilter(code *gojq.Code, data json.RawMessage) bool {
	if code == nil {
		return true
	}

	var input any
	if err := json.Unmarshal(data, &input); err != nil {
		return false
	}

	iter := code.Run(input)
	v, ok := iter.Next()
	if !ok {
		return false
	}

	// Handle error from jq
	if _, isErr := v.(error); isErr {
		return false
	}

	// jq filter expressions return true/false
	if b, ok := v.(bool); ok {
		return b
	}

	// Non-nil result means match (for select-style filters)
	return v != nil
}

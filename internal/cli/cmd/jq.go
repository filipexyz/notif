package cmd

import (
	"encoding/json"

	"github.com/itchyny/gojq"
)

// compileJqFilter parses and compiles a jq filter expression.
// Supports $input variable to reference the emitted request data.
func compileJqFilter(filter string) (*gojq.Code, error) {
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}
	return gojq.Compile(query, gojq.WithVariables([]string{"$input"}))
}

// matchesJqFilter evaluates a compiled jq filter against JSON data.
// Returns true if the filter matches (expression evaluates to true or non-nil).
// If code is nil, returns true (no filter = match all).
// The inputData parameter is available as $input in the filter expression.
func matchesJqFilter(code *gojq.Code, data json.RawMessage, inputData json.RawMessage) bool {
	if code == nil {
		return true
	}

	var response any
	if err := json.Unmarshal(data, &response); err != nil {
		return false
	}

	// Parse input data for $input variable
	var input any
	if inputData != nil {
		if err := json.Unmarshal(inputData, &input); err != nil {
			input = nil
		}
	}

	iter := code.Run(response, input)
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

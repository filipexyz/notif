package schema

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/filipexyz/notif/internal/cli/output"
	"github.com/filipexyz/notif/internal/schema"
	"github.com/spf13/cobra"
)

var (
	validateJSON   bool
	validateStrict bool
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a schema file",
		Long: `Validate a schema file without publishing.

This command checks:
  - Valid YAML/JSON syntax
  - Required fields present (name, version)
  - Valid field types
  - Valid version format (semver)
  - No circular references
  - Field names are valid identifiers

Examples:
  # Validate a schema
  notif schema validate ./agent.yaml

  # Validate with JSON output
  notif schema validate ./agent.yaml --json

  # Strict mode (warnings count as errors)
  notif schema validate ./agent.yaml --strict`,
		Args: cobra.ExactArgs(1),
		Run:  runValidate,
	}

	cmd.Flags().BoolVar(&validateJSON, "json", false, "output in JSON format")
	cmd.Flags().BoolVar(&validateStrict, "strict", false, "fail on warnings too")

	return cmd
}

func runValidate(cmd *cobra.Command, args []string) {
	out := output.New(validateJSON)
	filePath := args[0]

	// Parse schema
	s, err := schema.Parse(filePath)
	if err != nil {
		if validateJSON {
			out.JSON(map[string]any{
				"valid": false,
				"errors": []map[string]string{
					{"message": err.Error()},
				},
			})
		} else {
			out.Error("Failed to parse schema: %v", err)
		}
		os.Exit(1)
	}

	// Validate schema
	result := schema.Validate(s, validateStrict)

	// Output results
	if validateJSON {
		outputJSON(result)
	} else {
		outputText(out, result)
	}

	// Exit with error code if invalid
	if !result.Valid {
		os.Exit(1)
	}
}

func outputJSON(result *schema.ValidationResult) {
	type errorOutput struct {
		Field   string `json:"field,omitempty"`
		Message string `json:"message"`
	}

	data := map[string]any{
		"valid": result.Valid,
	}

	if len(result.Errors) > 0 {
		errors := make([]errorOutput, len(result.Errors))
		for i, e := range result.Errors {
			errors[i] = errorOutput{Field: e.Field, Message: e.Message}
		}
		data["errors"] = errors
	}

	if len(result.Warnings) > 0 {
		warnings := make([]errorOutput, len(result.Warnings))
		for i, w := range result.Warnings {
			warnings[i] = errorOutput{Field: w.Field, Message: w.Message}
		}
		data["warnings"] = warnings
	}

	output, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(output))
}

func outputText(out *output.Output, result *schema.ValidationResult) {
	if result.Valid {
		out.Success("Schema is valid")
	} else {
		fmt.Println("✗ Schema validation failed")
		fmt.Println()
	}

	// Print errors
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			if e.Field != "" {
				fmt.Printf("✗ Error in field %q: %s\n", e.Field, e.Message)
			} else {
				fmt.Printf("✗ Error: %s\n", e.Message)
			}
		}
		fmt.Println()
	}

	// Print warnings
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			if w.Field != "" {
				fmt.Printf("⚠ Warning in field %q: %s\n", w.Field, w.Message)
			} else {
				fmt.Printf("⚠ Warning: %s\n", w.Message)
			}
		}
		fmt.Println()
	}

	// Summary
	if !result.Valid {
		fmt.Printf("Found %d error(s)", len(result.Errors))
		if len(result.Warnings) > 0 {
			fmt.Printf(" and %d warning(s)", len(result.Warnings))
		}
		fmt.Println()
	} else if len(result.Warnings) > 0 {
		fmt.Printf("Schema is valid with %d warning(s)\n", len(result.Warnings))
	}
}

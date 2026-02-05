package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/filipexyz/notif/internal/cli/display"
	"github.com/filipexyz/notif/internal/codegen"
	"github.com/filipexyz/notif/pkg/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var schemasCmd = &cobra.Command{
	Use:   "schemas",
	Short: "Manage event schemas",
	Long: `Create, list, validate, and manage JSON schemas for event validation.

Schemas can include custom display configurations using the x-notif-display extension.
This controls how events are rendered in 'notif subscribe'.

Example schema with display config:
  {
    "type": "object",
    "x-notif-display": {
      "template": "{{.data.status | color \"green\"}} - {{.data.message}}"
    },
    "properties": {
      "status": { "type": "string" },
      "message": { "type": "string" }
    }
  }

Display config options:
  template      - Go template string with color/formatting functions
  topicPattern  - Extract variables from topic (e.g., "orders.{action}.{id}")
  fields        - Table mode with aligned columns
  conditions    - Conditional formatting based on jq expressions

Available template functions:
  Colors:    color, rgb, bg, bold, dim, italic, underline
  Strings:   upper, lower, truncate, trim, replace
  Format:    printf, json, time
  Logic:     eq, ne, gt, lt, contains, default

Examples:
  # Simple template
  "template": "{{.data.name | bold}} - {{.data.status}}"

  # With colors
  "template": "{{.data.level | color \"red\"}} {{.data.message}}"

  # Topic extraction
  "topicPattern": "orders.{action}.{id}",
  "template": "[{{.topic.action | upper}}] Order #{{.topic.id}}"

  # Conditional colors
  "conditions": [
    { "when": ".data.status == \"error\"", "color": "red" },
    { "when": ".data.status == \"ok\"", "color": "green" }
  ]`,
}

// SchemaDefinition represents the YAML schema file structure.
type SchemaDefinition struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Topic       string   `yaml:"topic" json:"topic"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	Schema interface{} `yaml:"schema" json:"schema"`

	Validation *ValidationConfig `yaml:"validation,omitempty" json:"validation,omitempty"`

	Compatibility string `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`

	Examples []interface{} `yaml:"examples,omitempty" json:"examples,omitempty"`
}

// ValidationConfig holds validation settings.
type ValidationConfig struct {
	Mode      string `yaml:"mode" json:"mode"`
	OnInvalid string `yaml:"onInvalid" json:"on_invalid"`
}

var schemasPushCmd = &cobra.Command{
	Use:   "push <file.yaml>",
	Short: "Push a schema from a YAML file",
	Long: `Push a schema definition from a YAML file to the server.
The file should contain the schema name, topic pattern, version, and JSON schema.

Example YAML file:
  name: order-placed
  version: "1.0.0"
  description: Schema for order.placed events
  topic: orders.placed

  schema:
    type: object
    required: [orderId, amount]
    properties:
      orderId:
        type: string
      amount:
        type: number

  validation:
    mode: strict
    onInvalid: reject

Examples:
  notif schemas push order-placed.yaml
  notif schemas push ./schemas/*.yaml`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()

		pushed := false
		for _, file := range args {
			if err := pushSchemaFile(c, file); err != nil {
				out.Error("Failed to push %s: %v", file, err)
				continue
			}
			pushed = true
		}

		// Clear cache so pushed schema displays are available immediately
		if pushed {
			clearSchemaCache()
		}
	},
}

func pushSchemaFile(c *client.Client, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var def SchemaDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return err
	}

	// First, try to get existing schema
	existing, err := c.SchemaGet(def.Name)
	if err != nil {
		// Schema doesn't exist, create it
		schema, err := c.SchemaCreate(client.CreateSchemaRequest{
			Name:         def.Name,
			TopicPattern: def.Topic,
			Description:  def.Description,
			Tags:         def.Tags,
		})
		if err != nil {
			return err
		}
		out.Success("Created schema: %s", schema.Name)
	} else {
		// Schema exists, update if needed
		if existing.TopicPattern != def.Topic || existing.Description != def.Description {
			_, err := c.SchemaUpdate(def.Name, client.UpdateSchemaRequest{
				TopicPattern: def.Topic,
				Description:  def.Description,
				Tags:         def.Tags,
			})
			if err != nil {
				return err
			}
			out.Info("Updated schema: %s", def.Name)
		}
	}

	// Create version if specified
	if def.Version != "" && def.Schema != nil {
		validationMode := "strict"
		onInvalid := "reject"
		if def.Validation != nil {
			if def.Validation.Mode != "" {
				validationMode = def.Validation.Mode
			}
			if def.Validation.OnInvalid != "" {
				onInvalid = def.Validation.OnInvalid
			}
		}

		// Convert schema from interface{} to json.RawMessage
		schemaJSON, err := json.Marshal(def.Schema)
		if err != nil {
			return err
		}

		var examples json.RawMessage
		if len(def.Examples) > 0 {
			examples, _ = json.Marshal(def.Examples)
		}

		version, err := c.SchemaVersionCreate(def.Name, client.CreateSchemaVersionRequest{
			Version:        def.Version,
			Schema:         schemaJSON,
			ValidationMode: validationMode,
			OnInvalid:      onInvalid,
			Compatibility:  def.Compatibility,
			Examples:       examples,
		})
		if err != nil {
			// Version might already exist
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "already exists") {
				out.Info("Version %s already exists for %s", def.Version, def.Name)
			} else {
				return err
			}
		} else {
			out.Success("Created version %s for schema %s", version.Version, def.Name)
		}
	}

	return nil
}

var schemasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all schemas",
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.SchemaList()
		if err != nil {
			out.Error("Failed to list schemas: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No schemas configured")
			return
		}

		out.Header("Schemas")
		out.Divider()

		for _, s := range result.Schemas {
			out.Info("%s", s.Name)
			out.KeyValue("Topic", s.TopicPattern)
			if s.Description != "" {
				out.KeyValue("Description", s.Description)
			}
			if s.LatestVersion != nil {
				out.KeyValue("Latest Version", s.LatestVersion.Version)
				out.KeyValue("Validation", s.LatestVersion.ValidationMode)
			}
			if len(s.Tags) > 0 {
				out.KeyValue("Tags", strings.Join(s.Tags, ", "))
			}
			out.Divider()
		}
	},
}

var (
	getSchemaOnly bool
)

var schemasGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get schema details",
	Long: `Get schema details. Use --schema to output only the JSON Schema (for piping).

Examples:
  notif schemas get order-placed
  notif schemas get order-placed --schema
  notif schemas get order-placed --schema | jq '.properties'`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		schema, err := c.SchemaGet(args[0])
		if err != nil {
			out.Error("Failed to get schema: %v", err)
			return
		}

		// Output only JSON Schema for piping
		if getSchemaOnly {
			if schema.LatestVersion == nil {
				out.Error("Schema has no versions")
				return
			}
			fmt.Println(string(schema.LatestVersion.Schema))
			return
		}

		if jsonOutput {
			out.JSON(schema)
			return
		}

		out.Header("Schema: " + schema.Name)
		out.KeyValue("ID", schema.ID)
		out.KeyValue("Topic Pattern", schema.TopicPattern)
		if schema.Description != "" {
			out.KeyValue("Description", schema.Description)
		}
		if len(schema.Tags) > 0 {
			out.KeyValue("Tags", strings.Join(schema.Tags, ", "))
		}
		out.KeyValue("Created", schema.CreatedAt.Format("2006-01-02 15:04:05"))

		if schema.LatestVersion != nil {
			out.Divider()
			out.Info("Latest Version: %s", schema.LatestVersion.Version)
			out.KeyValue("Validation Mode", schema.LatestVersion.ValidationMode)
			out.KeyValue("On Invalid", schema.LatestVersion.OnInvalid)
			out.KeyValue("Fingerprint", schema.LatestVersion.Fingerprint[:16]+"...")
		}
	},
}

var schemasDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a schema",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		if err := c.SchemaDelete(args[0]); err != nil {
			out.Error("Failed to delete schema: %v", err)
			return
		}

		// Clear cache so deleted schema is removed immediately
		clearSchemaCache()

		if jsonOutput {
			out.JSON(map[string]string{"status": "deleted"})
			return
		}

		out.Success("Schema deleted: %s", args[0])
	},
}

var schemasValidateCmd = &cobra.Command{
	Use:   "validate <schema-name> [data]",
	Short: "Validate data against a schema",
	Long: `Validate JSON data against a schema. Data can be provided as:
- An argument: notif schemas validate order-placed '{"orderId":"123"}'
- From stdin: echo '{"orderId":"123"}' | notif schemas validate order-placed
- From a file: notif schemas validate order-placed @data.json`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		schemaName := args[0]
		var data []byte
		var err error

		if len(args) > 1 {
			dataArg := args[1]
			if strings.HasPrefix(dataArg, "@") {
				// Read from file
				data, err = os.ReadFile(dataArg[1:])
				if err != nil {
					out.Error("Failed to read file: %v", err)
					return
				}
			} else {
				data = []byte(dataArg)
			}
		} else {
			// Read from stdin
			data, err = os.ReadFile("/dev/stdin")
			if err != nil {
				out.Error("Failed to read stdin: %v", err)
				return
			}
		}

		c := getClient()
		result, err := c.SchemaValidate(schemaName, json.RawMessage(data))
		if err != nil {
			out.Error("Failed to validate: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Valid {
			out.Success("Valid")
			out.KeyValue("Schema", result.Schema)
			out.KeyValue("Version", result.Version)
		} else {
			out.Error("Invalid")
			out.KeyValue("Schema", result.Schema)
			out.KeyValue("Version", result.Version)
			out.Divider()
			for _, e := range result.Errors {
				out.Error("  %s: %s", e.Field, e.Message)
			}
		}
	},
}

var schemasVersionsCmd = &cobra.Command{
	Use:   "versions <schema-name>",
	Short: "List versions of a schema",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		result, err := c.SchemaVersionList(args[0])
		if err != nil {
			out.Error("Failed to list versions: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(result)
			return
		}

		if result.Count == 0 {
			out.Info("No versions for schema %s", args[0])
			return
		}

		out.Header("Versions for " + args[0])
		out.Divider()

		for _, v := range result.Versions {
			latest := ""
			if v.IsLatest {
				latest = " (latest)"
			}
			out.Info("%s%s", v.Version, latest)
			out.KeyValue("Validation", v.ValidationMode)
			out.KeyValue("On Invalid", v.OnInvalid)
			out.KeyValue("Created", v.CreatedAt.Format("2006-01-02 15:04:05"))
			out.Divider()
		}
	},
}

var schemasForTopicCmd = &cobra.Command{
	Use:   "for-topic <topic>",
	Short: "Find schema for a topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		c := getClient()
		schema, err := c.SchemaForTopic(args[0])
		if err != nil {
			out.Error("Failed to find schema: %v", err)
			return
		}

		if schema == nil {
			if jsonOutput {
				out.JSON(map[string]interface{}{"schema": nil})
				return
			}
			out.Info("No schema found for topic: %s", args[0])
			return
		}

		if jsonOutput {
			out.JSON(schema)
			return
		}

		out.Success("Found schema: %s", schema.Name)
		out.KeyValue("Topic Pattern", schema.TopicPattern)
		if schema.LatestVersion != nil {
			out.KeyValue("Version", schema.LatestVersion.Version)
			out.KeyValue("Validation", schema.LatestVersion.ValidationMode)
		}
	},
}

var (
	generateConfigFile string
	generateDryRun     bool
	initAllSchemas     bool
)

var schemasGenerateCmd = &cobra.Command{
	Use:   "generate [schema-name]",
	Short: "Generate typed code from schemas",
	Long: `Generate typed code (TypeScript/Go) from notif.sh JSON Schemas.

Reads configuration from .notif.yaml and generates code for all configured schemas.
Optionally specify a schema name to generate code for only that schema.

Examples:
  notif schemas generate                    # Generate all schemas
  notif schemas generate order-placed       # Generate specific schema
  notif schemas generate --dry-run          # Preview what would be generated
  notif schemas generate -c custom.yaml     # Use custom config file`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Find or use specified config file
		configPath := generateConfigFile
		if configPath == "" {
			var err error
			configPath, err = codegen.FindConfig()
			if err != nil {
				out.Error("No .notif.yaml found. Run 'notif schemas init' to create one.")
				return
			}
		}

		// Load config
		cfg, err := codegen.LoadConfig(configPath)
		if err != nil {
			out.Error("Failed to load config: %v", err)
			return
		}

		// Get API key for fetching schemas from server
		apiKey := os.Getenv("NOTIF_API_KEY")
		if apiKey == "" {
			// Check if all schemas are local files (or if using "schemas: all")
			needsServer := cfg.Schemas.All
			if !needsServer {
				for _, s := range cfg.Schemas.Entries {
					if s.File == "" {
						needsServer = true
						break
					}
				}
			}
			if needsServer {
				out.Error("No API key configured. Set NOTIF_API_KEY or run 'notif auth <key>'.")
				return
			}
		}

		// Create client (may be nil if all schemas are local)
		var c *client.Client
		if apiKey != "" {
			server := cfg.Server
			if server == "" {
				server = serverURL
			}
			c = client.New(apiKey, client.WithServer(server))
		}

		// Create generator
		opts := []codegen.GeneratorOption{
			codegen.WithDryRun(generateDryRun),
			codegen.WithProgressCallback(func(msg string) {
				out.Info(msg)
			}),
		}

		gen := codegen.NewGenerator(cfg, c, configPath, opts...)

		// Filter schema if specified
		var filterSchema string
		if len(args) > 0 {
			filterSchema = args[0]
		}

		// Generate
		results, err := gen.Generate(filterSchema)
		if err != nil {
			out.Error("Generation failed: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(results)
			return
		}

		// Summary
		generated, failed, _ := codegen.CountResults(results)
		out.Divider()
		if generateDryRun {
			out.Info("Dry run: would generate %d files", generated)
		} else {
			out.Success("Generated %d files", generated)
		}
		if failed > 0 {
			out.Error("Failed: %d", failed)
			for _, r := range results {
				if r.Error != nil {
					out.Error("  %s: %v", r.Schema, r.Error)
				}
			}
		}
	},
}

var schemasInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize schema codegen configuration",
	Long: `Create a .notif.yaml configuration file for schema code generation.

Examples:
  notif schemas init          # Create config with example schema
  notif schemas init --all    # Create config with all schemas from server`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath := ".notif.yaml"

		// Check if config already exists
		if _, err := os.Stat(configPath); err == nil {
			out.Error("Config file already exists: %s", configPath)
			return
		}

		cfg := codegen.CreateDefaultConfig()

		// If --all flag, fetch schemas from server
		if initAllSchemas {
			apiKey := os.Getenv("NOTIF_API_KEY")
			if apiKey == "" {
				out.Error("No API key configured. Set NOTIF_API_KEY to fetch schemas.")
				return
			}

			c := getClient()
			result, err := c.SchemaList()
			if err != nil {
				out.Error("Failed to list schemas: %v", err)
				return
			}

			cfg.Schemas.Entries = nil
			for _, s := range result.Schemas {
				cfg.Schemas.Entries = append(cfg.Schemas.Entries, codegen.SchemaEntry{Name: s.Name})
			}

			if len(cfg.Schemas.Entries) == 0 {
				out.Warn("No schemas found on server. Using example placeholder.")
				cfg.Schemas.Entries = []codegen.SchemaEntry{{Name: "example-schema"}}
			}
		}

		if err := codegen.WriteConfig(cfg, configPath); err != nil {
			out.Error("Failed to write config: %v", err)
			return
		}

		if jsonOutput {
			out.JSON(map[string]string{"path": configPath})
			return
		}

		absPath, _ := filepath.Abs(configPath)
		out.Success("Created %s", absPath)
		out.Info("Edit the file to configure your schemas, then run:")
		out.Info("  notif schemas generate")
	},
}

var (
	createTopic       string
	createVersion     string
	createDescription string
	editVersion       string
	cacheRefresh      bool
	cacheClear        bool
)

var schemasCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new schema with JSON Schema from stdin",
	Long: `Create a new schema. Reads JSON Schema from stdin.

Examples:
  # Basic schema
  echo '{"type": "object", "properties": {"id": {"type": "string"}}}' | notif schemas create order-placed --topic "orders.placed"

  # Schema with custom display for 'notif subscribe'
  cat <<'EOF' | notif schemas create payment --topic "payments.*"
  {
    "type": "object",
    "x-notif-display": {
      "template": "💰 {{.data.amount | printf \"$%.2f\"}} - {{.data.status | color \"green\"}}"
    },
    "properties": {
      "amount": { "type": "number" },
      "status": { "type": "string" }
    }
  }
  EOF

  # Schema with conditional colors
  cat <<'EOF' | notif schemas create logs --topic "logs.*"
  {
    "type": "object",
    "x-notif-display": {
      "template": "[{{.data.level}}] {{.data.message}}",
      "conditions": [
        { "when": ".data.level == \"error\"", "color": "red" },
        { "when": ".data.level == \"warn\"", "color": "yellow" },
        { "when": ".data.level == \"info\"", "color": "blue" }
      ]
    },
    "properties": {
      "level": { "type": "string" },
      "message": { "type": "string" }
    }
  }
  EOF`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		if createTopic == "" {
			out.Error("--topic is required")
			return
		}

		// Read JSON Schema from stdin
		schemaData, err := io.ReadAll(os.Stdin)
		if err != nil {
			out.Error("Failed to read stdin: %v", err)
			return
		}

		if len(schemaData) == 0 {
			out.Error("No JSON Schema provided on stdin")
			return
		}

		// Validate it's valid JSON
		var schemaJSON json.RawMessage
		if err := json.Unmarshal(schemaData, &schemaJSON); err != nil {
			out.Error("Invalid JSON: %v", err)
			return
		}

		c := getClient()

		// Create schema
		schema, err := c.SchemaCreate(client.CreateSchemaRequest{
			Name:         args[0],
			TopicPattern: createTopic,
			Description:  createDescription,
		})
		if err != nil {
			out.Error("Failed to create schema: %v", err)
			return
		}

		// Create initial version
		version := createVersion
		if version == "" {
			version = "1.0.0"
		}

		_, err = c.SchemaVersionCreate(args[0], client.CreateSchemaVersionRequest{
			Version:        version,
			Schema:         schemaJSON,
			ValidationMode: "strict",
			OnInvalid:      "reject",
		})
		if err != nil {
			out.Error("Schema created but failed to create version: %v", err)
			return
		}

		// Clear cache so new schema display is available immediately
		clearSchemaCache()

		if jsonOutput {
			out.JSON(schema)
			return
		}

		out.Success("Created schema: %s (version %s)", args[0], version)
	},
}

var schemasEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Update schema with JSON Schema from stdin",
	Long: `Update a schema with a new version. Reads JSON Schema from stdin.

Examples:
  cat schema.json | notif schemas edit order-placed
  notif schemas get order-placed --schema | jq '.properties.amount.type = "integer"' | notif schemas edit order-placed
  notif schemas edit order-placed --version 2.0.0 < schema.json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		// Read JSON Schema from stdin
		schemaData, err := io.ReadAll(os.Stdin)
		if err != nil {
			out.Error("Failed to read stdin: %v", err)
			return
		}

		if len(schemaData) == 0 {
			out.Error("No JSON Schema provided on stdin")
			return
		}

		// Validate it's valid JSON
		var schemaJSON json.RawMessage
		if err := json.Unmarshal(schemaData, &schemaJSON); err != nil {
			out.Error("Invalid JSON: %v", err)
			return
		}

		c := getClient()

		// Get current schema to determine next version
		schema, err := c.SchemaGet(args[0])
		if err != nil {
			out.Error("Failed to get schema: %v", err)
			return
		}

		// Determine version
		version := editVersion
		if version == "" {
			if schema.LatestVersion != nil {
				version = bumpPatchVersion(schema.LatestVersion.Version)
			} else {
				version = "1.0.0"
			}
		}

		// Create new version
		_, err = c.SchemaVersionCreate(args[0], client.CreateSchemaVersionRequest{
			Version:        version,
			Schema:         schemaJSON,
			ValidationMode: "strict",
			OnInvalid:      "reject",
		})
		if err != nil {
			out.Error("Failed to create version: %v", err)
			return
		}

		// Clear cache so updated schema display is available immediately
		clearSchemaCache()

		if jsonOutput {
			out.JSON(map[string]string{"schema": args[0], "version": version})
			return
		}

		out.Success("Updated schema: %s (version %s)", args[0], version)
	},
}

// clearSchemaCache clears the local schema cache after modifications.
func clearSchemaCache() {
	c := getClient()
	loader := display.NewConfigLoader(c)
	_ = loader.ClearCache() // Ignore errors, cache clearing is best-effort
}

// bumpPatchVersion increments the patch version (1.0.0 -> 1.0.1)
func bumpPatchVersion(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return v + ".1"
	}
	var patch int
	fmt.Sscanf(parts[2], "%d", &patch)
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}

var schemasCacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage local schema cache",
	Long: `View, refresh, or clear the local schema cache used for display configurations.

Examples:
  notif schemas cache              # Show cache status
  notif schemas cache --refresh    # Force refresh from server
  notif schemas cache --clear      # Remove local cache`,
	Run: func(cmd *cobra.Command, args []string) {
		c := getClient()
		loader := display.NewConfigLoader(c)

		// Handle --clear flag
		if cacheClear {
			if err := loader.ClearCache(); err != nil {
				out.Error("Failed to clear cache: %v", err)
				return
			}
			if jsonOutput {
				out.JSON(map[string]string{"status": "cleared"})
				return
			}
			out.Success("Cache cleared")
			return
		}

		// Handle --refresh flag
		if cacheRefresh {
			if cfg.APIKey == "" {
				out.Error("No API key configured. Run 'notif auth <key>' first.")
				return
			}
			if err := loader.Refresh(cmd.Context()); err != nil {
				out.Error("Failed to refresh cache: %v", err)
				return
			}
			if jsonOutput {
				out.JSON(map[string]string{"status": "refreshed"})
				return
			}
			out.Success("Cache refreshed")
		}

		// Load cache to show status
		_ = loader.Load(cmd.Context())

		info := loader.CacheInfo()
		if info == nil {
			if jsonOutput {
				out.JSON(map[string]interface{}{"cached": false})
				return
			}
			out.Info("No cache available")
			out.Info("Run 'notif schemas cache --refresh' to populate the cache")
			return
		}

		if jsonOutput {
			out.JSON(map[string]interface{}{
				"cached":       true,
				"server":       info.Server,
				"last_sync":    info.LastSync,
				"ttl":          info.TTL,
				"schema_count": info.SchemaCount,
			})
			return
		}

		out.Header("Schema Cache")
		out.KeyValue("Cache directory", "~/.notif/cache/schemas/")
		out.KeyValue("Server", info.Server)
		out.KeyValue("Last sync", formatRelativeTime(info.LastSync))
		out.KeyValue("Schemas", fmt.Sprintf("%d", info.SchemaCount))
		out.KeyValue("TTL", fmt.Sprintf("%d seconds", info.TTL))

		// Show schemas with display configs
		schemas := loader.GetAllSchemas()
		displayCount := 0
		for _, s := range schemas {
			if s.Display != nil {
				displayCount++
			}
		}
		out.KeyValue("With display config", fmt.Sprintf("%d", displayCount))
	},
}

// formatRelativeTime formats a timestamp as a relative time string.
func formatRelativeTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}

	duration := time.Since(t)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func init() {
	schemasCmd.AddCommand(schemasPushCmd)
	schemasCmd.AddCommand(schemasListCmd)
	schemasCmd.AddCommand(schemasGetCmd)
	schemasCmd.AddCommand(schemasDeleteCmd)
	schemasCmd.AddCommand(schemasValidateCmd)
	schemasCmd.AddCommand(schemasVersionsCmd)
	schemasCmd.AddCommand(schemasForTopicCmd)
	schemasCmd.AddCommand(schemasGenerateCmd)
	schemasCmd.AddCommand(schemasInitCmd)
	schemasCmd.AddCommand(schemasCreateCmd)
	schemasCmd.AddCommand(schemasEditCmd)
	schemasCmd.AddCommand(schemasCacheCmd)

	// Get command flags
	schemasGetCmd.Flags().BoolVar(&getSchemaOnly, "schema", false, "output only the JSON Schema (for piping)")

	// Create command flags
	schemasCreateCmd.Flags().StringVar(&createTopic, "topic", "", "topic pattern (required)")
	schemasCreateCmd.Flags().StringVar(&createVersion, "version", "", "initial version (default 1.0.0)")
	schemasCreateCmd.Flags().StringVar(&createDescription, "description", "", "schema description")
	schemasCreateCmd.MarkFlagRequired("topic")

	// Edit command flags
	schemasEditCmd.Flags().StringVar(&editVersion, "version", "", "version number (default: auto-increment patch)")

	// Generate command flags
	schemasGenerateCmd.Flags().StringVarP(&generateConfigFile, "config", "c", "", "config file (default .notif.yaml)")
	schemasGenerateCmd.Flags().BoolVar(&generateDryRun, "dry-run", false, "show what would be generated without writing files")

	// Init command flags
	schemasInitCmd.Flags().BoolVar(&initAllSchemas, "all", false, "include all schemas from server")

	// Cache command flags
	schemasCacheCmd.Flags().BoolVar(&cacheRefresh, "refresh", false, "force refresh from server")
	schemasCacheCmd.Flags().BoolVar(&cacheClear, "clear", false, "remove local cache")

	rootCmd.AddCommand(schemasCmd)
}

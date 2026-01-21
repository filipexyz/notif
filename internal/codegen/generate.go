package codegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/filipexyz/notif/pkg/client"
	"gopkg.in/yaml.v3"
)

// Generator orchestrates code generation.
type Generator struct {
	config     *Config
	client     *client.Client
	tsGen      *TypeScriptGenerator
	goGen      *GoGenerator
	configDir  string // Directory containing the config file
	dryRun     bool
	verbose    bool
	onProgress func(msg string) // Callback for progress messages
}

// GeneratorOption configures the generator.
type GeneratorOption func(*Generator)

// WithDryRun enables dry run mode.
func WithDryRun(dryRun bool) GeneratorOption {
	return func(g *Generator) {
		g.dryRun = dryRun
	}
}

// WithVerbose enables verbose output.
func WithVerbose(verbose bool) GeneratorOption {
	return func(g *Generator) {
		g.verbose = verbose
	}
}

// WithProgressCallback sets a callback for progress messages.
func WithProgressCallback(cb func(msg string)) GeneratorOption {
	return func(g *Generator) {
		g.onProgress = cb
	}
}

// NewGenerator creates a new generator.
func NewGenerator(config *Config, client *client.Client, configPath string, opts ...GeneratorOption) *Generator {
	g := &Generator{
		config:    config,
		client:    client,
		configDir: filepath.Dir(configPath),
		tsGen:     NewTypeScriptGenerator(config.Options.TypeScript),
		goGen:     NewGoGenerator(config.Options.Go),
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// GenerateResult contains the result of code generation.
type GenerateResult struct {
	Schema    string
	Language  string
	FilePath  string
	Generated bool
	Error     error
}

// Generate generates code for all configured schemas.
func (g *Generator) Generate(filterSchema string) ([]GenerateResult, error) {
	var results []GenerateResult

	// Get schema entries to process
	entries, err := g.getSchemaEntries()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Filter by schema name if specified
		if filterSchema != "" && entry.Name != filterSchema {
			continue
		}

		// Get schema data
		schema, err := g.fetchSchema(entry)
		if err != nil {
			results = append(results, GenerateResult{
				Schema: entry.Name,
				Error:  err,
			})
			continue
		}

		// Generate for each configured language
		languages := g.config.GetLanguagesForSchema(entry)
		for _, lang := range languages {
			result := g.generateForLanguage(schema, lang)
			results = append(results, result)
		}
	}

	// Generate barrel files
	if !g.dryRun {
		if err := g.generateBarrelFiles(results); err != nil {
			g.log("Warning: failed to generate barrel files: %v", err)
		}
	}

	return results, nil
}

// getSchemaEntries returns the list of schemas to generate.
// If config has "schemas: all", fetches all schemas from the server.
func (g *Generator) getSchemaEntries() ([]SchemaEntry, error) {
	if !g.config.Schemas.All {
		return g.config.Schemas.Entries, nil
	}

	// Fetch all schemas from server
	if g.client == nil {
		return nil, fmt.Errorf("cannot use 'schemas: all' without API key")
	}

	g.log("Fetching all schemas from server...")
	result, err := g.client.SchemaList()
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	var entries []SchemaEntry
	for _, s := range result.Schemas {
		if s.LatestVersion != nil { // Only include schemas with versions
			entries = append(entries, SchemaEntry{Name: s.Name})
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no schemas with versions found on server")
	}

	g.log("Found %d schemas", len(entries))
	return entries, nil
}

func (g *Generator) fetchSchema(entry SchemaEntry) (*Schema, error) {
	var schemaJSON []byte
	var topic, version string

	if entry.File != "" {
		// Load from local file
		filePath := entry.File
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(g.configDir, filePath)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read local schema file: %w", err)
		}

		// Parse YAML file (like the push command does)
		var def struct {
			Name    string      `yaml:"name"`
			Version string      `yaml:"version"`
			Topic   string      `yaml:"topic"`
			Schema  interface{} `yaml:"schema"`
		}
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("failed to parse schema file: %w", err)
		}

		schemaJSON, err = json.Marshal(def.Schema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema: %w", err)
		}

		topic = def.Topic
		version = def.Version
	} else {
		// Fetch from server
		g.log("Fetching schema %s from server...", entry.Name)

		s, err := g.client.SchemaGet(entry.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch schema: %w", err)
		}

		if s.LatestVersion == nil {
			return nil, fmt.Errorf("schema %s has no versions", entry.Name)
		}

		schemaJSON = s.LatestVersion.Schema
		topic = s.TopicPattern
		version = s.LatestVersion.Version
	}

	// Parse JSON Schema to IR
	schema, err := ParseJSONSchema(schemaJSON, entry.Name, topic, version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return schema, nil
}

func (g *Generator) generateForLanguage(schema *Schema, lang string) GenerateResult {
	result := GenerateResult{
		Schema:   schema.Name,
		Language: lang,
	}

	var code string
	var filename string
	var outDir string
	var err error

	switch lang {
	case "typescript":
		code, err = g.tsGen.Generate(schema)
		filename = toSnakeCase(schema.Name) + ".ts"
		outDir = g.config.Output.TypeScript
	case "go":
		code, err = g.goGen.GenerateWithImports(schema)
		filename = toSnakeCase(schema.Name) + ".go"
		outDir = g.config.Output.Go
	default:
		result.Error = fmt.Errorf("unsupported language: %s", lang)
		return result
	}

	if err != nil {
		result.Error = err
		return result
	}

	// Resolve output path
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(g.configDir, outDir)
	}
	result.FilePath = filepath.Join(outDir, filename)

	if g.dryRun {
		g.log("Would generate: %s", result.FilePath)
		result.Generated = true
		return result
	}

	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create output directory: %w", err)
		return result
	}

	// Write file
	if err := os.WriteFile(result.FilePath, []byte(code), 0644); err != nil {
		result.Error = fmt.Errorf("failed to write file: %w", err)
		return result
	}

	g.log("Generated: %s", result.FilePath)
	result.Generated = true
	return result
}

func (g *Generator) generateBarrelFiles(results []GenerateResult) error {
	// Group successful TypeScript results
	var tsSchemas []string
	for _, r := range results {
		if r.Language == "typescript" && r.Generated && r.Error == nil {
			tsSchemas = append(tsSchemas, r.Schema)
		}
	}

	if len(tsSchemas) > 0 && g.config.Output.TypeScript != "" {
		outDir := g.config.Output.TypeScript
		if !filepath.IsAbs(outDir) {
			outDir = filepath.Join(g.configDir, outDir)
		}

		barrelCode := g.tsGen.GenerateBarrelFile(tsSchemas)
		barrelPath := filepath.Join(outDir, "index.ts")

		if err := os.WriteFile(barrelPath, []byte(barrelCode), 0644); err != nil {
			return fmt.Errorf("failed to write barrel file: %w", err)
		}

		g.log("Generated: %s", barrelPath)
	}

	return nil
}

func (g *Generator) log(format string, args ...interface{}) {
	if g.onProgress != nil {
		g.onProgress(fmt.Sprintf(format, args...))
	}
}

// CountResults returns counts of generated, failed, and skipped schemas.
func CountResults(results []GenerateResult) (generated, failed, skipped int) {
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else if r.Generated {
			generated++
		} else {
			skipped++
		}
	}
	return
}

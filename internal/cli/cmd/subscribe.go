package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/filipexyz/notif/internal/cli/display"
	"github.com/filipexyz/notif/pkg/client"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
)

var (
	subscribeGroup   string
	subscribeFrom    string
	subscribeNoAck   bool
	subscribeFilter  string
	subscribeOnce    bool
	subscribeCount   int
	subscribeTimeout time.Duration
	subscribeFormat  string
	subscribeFields  string
	subscribeNoColor bool
	subscribeNoCache bool
	subscribeOffline bool
	subscribeRaw     bool
)

var subscribeCmd = &cobra.Command{
	Use:   "subscribe <topics...>",
	Short: "Subscribe to topics",
	Long: `Subscribe to one or more topics and receive events in real-time.

Examples:
  notif subscribe orders.created
  notif subscribe "orders.*"
  notif subscribe orders.created users.signup
  notif subscribe --group processor "orders.*"

Filter and auto-exit:
  notif subscribe 'orders.*' --filter '.status == "completed"' --once
  notif subscribe 'orders.*' --filter '.amount > 100' --count 5 --timeout 30s

Custom display:
  notif subscribe 'orders.*' --format '{{.data.orderId}} - {{.data.status | color "green"}}'
  notif subscribe 'payments.*' --format '{{.topic}} {{.data.amount | printf "$%.2f"}}'
  notif subscribe 'logs.*' --fields "timestamp,topic,data.level,data.message"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if cfg.APIKey == "" {
			out.Error("No API key configured. Run 'notif auth <key>' first.")
			return
		}

		topics := args

		// Parse jq filter if provided
		var jqCode *gojq.Code
		if subscribeFilter != "" {
			code, err := compileJqFilter(subscribeFilter)
			if err != nil {
				out.Error("Invalid jq filter: %v", err)
				os.Exit(1)
			}
			jqCode = code
		}

		// Normalize --once to --count 1
		if subscribeOnce {
			subscribeCount = 1
		}

		c := getClient()

		// Set up context with optional timeout
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if subscribeTimeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), subscribeTimeout)
			defer cancel()
		}

		opts := client.SubscribeOptions{
			AutoAck: !subscribeNoAck,
			Group:   subscribeGroup,
			From:    subscribeFrom,
		}

		sub, err := c.Subscribe(ctx, topics, opts)
		if err != nil {
			out.Error("Failed to subscribe: %v", err)
			return
		}
		defer sub.Close()

		// Set up display renderer
		renderer := setupRenderer(ctx, c, topics)

		if !jsonOutput {
			out.Success("Subscribed to %v", topics)
			if subscribeGroup != "" {
				out.KeyValue("Group", subscribeGroup)
			}
			if subscribeFilter != "" {
				out.KeyValue("Filter", subscribeFilter)
			}
			if subscribeFormat != "" {
				out.KeyValue("Display", "custom template")
			} else if subscribeFields != "" {
				out.KeyValue("Display", "table mode")
			}
			if subscribeCount > 0 {
				out.KeyValue("Exit after", fmt.Sprintf("%d events", subscribeCount))
			}
			out.Info("Waiting for events... (Ctrl+C to exit)")
			out.Divider()
		}

		// Handle signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		matchCount := 0

		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					return
				}

				// Check filter (no $input for subscribe)
				if !matchesJqFilter(jqCode, event.Data, nil) {
					continue // skip non-matching events
				}

				// Render event
				if jsonOutput {
					out.Event(event.ID, event.Topic, event.Data, event.Timestamp)
				} else {
					output, err := renderer.RenderEvent(event.ID, event.Topic, event.Data, event.Timestamp)
					if err != nil {
						// Fallback to default format on error
						out.Event(event.ID, event.Topic, event.Data, event.Timestamp)
					} else {
						fmt.Println(output)
					}
				}
				matchCount++

				// Check exit condition
				if subscribeCount > 0 && matchCount >= subscribeCount {
					return
				}

			case err := <-sub.Errors():
				// Log error but don't exit - SDK will auto-reconnect
				if _, ok := err.(*client.ReconnectedError); ok {
					out.Success("Reconnected")
				} else {
					out.Warn("Connection error: %v (reconnecting...)", err)
				}

			case <-sigCh:
				if !jsonOutput {
					out.Info("Disconnecting...")
				}
				return

			case <-ctx.Done():
				if subscribeTimeout > 0 {
					out.Error("Timeout waiting for events")
					os.Exit(1)
				}
				return
			}
		}
	},
}

// setupRenderer creates the appropriate renderer manager based on config.
func setupRenderer(ctx context.Context, c *client.Client, topics []string) *display.RendererManager {
	// Create colorizer
	colorEnabled := !subscribeNoColor && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	colorizer := display.NewColorizer(colorEnabled)

	// Create renderer manager
	renderer := display.NewRendererManager(colorizer)

	// Raw mode: skip all custom display, use default renderer
	if subscribeRaw {
		return renderer
	}

	// Priority 1: CLI --format flag
	if subscribeFormat != "" {
		cfg := &display.DisplayConfig{Template: subscribeFormat}
		if err := renderer.SetDefaultConfig(cfg); err != nil {
			out.Warn("Invalid format template: %v", err)
		}
		return renderer
	}

	// Priority 2: CLI --fields flag (table mode)
	if subscribeFields != "" {
		fields := parseFieldsFlag(subscribeFields)
		cfg := &display.DisplayConfig{Fields: fields}
		if err := renderer.SetDefaultConfig(cfg); err != nil {
			out.Warn("Invalid fields config: %v", err)
		}
		return renderer
	}

	// Priority 3: Load project config (.notif.json) and add ALL topic configs
	projectCfg, err := display.LoadProjectConfig()
	if err != nil {
		out.Warn("Failed to load .notif.json: %v", err)
	}

	// Add all topic configs from .notif.json
	if projectCfg != nil && projectCfg.Display != nil && projectCfg.Display.Topics != nil {
		for pattern, cfg := range projectCfg.Display.Topics {
			if cfg != nil {
				if err := renderer.AddTopicConfig(pattern, cfg); err != nil {
					out.Warn("Failed to setup display for %s: %v", pattern, err)
				}
			}
		}
	}

	// Priority 4: Load schema cache (server configs)
	if !subscribeOffline || !subscribeNoCache {
		loader := display.NewConfigLoader(c)
		if subscribeNoCache {
			loader.WithNoCache()
		}
		if subscribeOffline {
			loader.WithOffline()
		}
		if err := loader.Load(ctx); err != nil {
			// Non-fatal, continue without schema configs
			if subscribeOffline {
				out.Warn("Failed to load schema cache: %v", err)
			}
		} else {
			// Add all schemas with display configs
			for _, schema := range loader.GetAllSchemas() {
				if schema.Display != nil {
					if err := renderer.AddTopicConfig(schema.TopicPattern, schema.Display); err != nil {
						out.Warn("Failed to setup display for schema %s: %v", schema.Name, err)
					}
				}
			}
		}
	}

	return renderer
}

// parseFieldsFlag parses the --fields flag into FieldConfig slice.
func parseFieldsFlag(fields string) []display.FieldConfig {
	parts := strings.Split(fields, ",")
	result := make([]display.FieldConfig, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, display.FieldConfig{
			Path: part,
		})
	}

	return result
}

func init() {
	subscribeCmd.Flags().StringVar(&subscribeGroup, "group", "", "consumer group name")
	subscribeCmd.Flags().StringVar(&subscribeFrom, "from", "latest", "start position (latest, beginning)")
	subscribeCmd.Flags().BoolVar(&subscribeNoAck, "no-auto-ack", false, "disable automatic acknowledgment")
	subscribeCmd.Flags().StringVar(&subscribeFilter, "filter", "", "jq expression to filter events")
	subscribeCmd.Flags().BoolVar(&subscribeOnce, "once", false, "exit after first matching event")
	subscribeCmd.Flags().IntVar(&subscribeCount, "count", 0, "exit after N matching events")
	subscribeCmd.Flags().DurationVar(&subscribeTimeout, "timeout", 0, "timeout waiting for events")

	// Display options
	subscribeCmd.Flags().StringVar(&subscribeFormat, "format", "", "custom template for event display")
	subscribeCmd.Flags().StringVar(&subscribeFields, "fields", "", "comma-separated fields for table display")
	subscribeCmd.Flags().BoolVar(&subscribeNoColor, "no-color", false, "disable colored output")
	subscribeCmd.Flags().BoolVar(&subscribeNoCache, "no-cache", false, "ignore schema cache, always fetch from server")
	subscribeCmd.Flags().BoolVar(&subscribeOffline, "offline", false, "use only local cache (error if not available)")
	subscribeCmd.Flags().BoolVar(&subscribeRaw, "raw", false, "disable custom display, show raw format (timestamp topic json)")

	rootCmd.AddCommand(subscribeCmd)
}

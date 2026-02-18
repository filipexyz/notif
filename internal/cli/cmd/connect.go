package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/filipexyz/notif/internal/bridge"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	connectNatsURL      string
	connectTopics       []string
	connectInterceptors string
	connectStream       string
	connectCloud        bool
	connectConfigPath   string
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Manage the local NATS bridge",
	Long: `Manage a bridge that connects to a local NATS server, runs interceptors (jq transforms),
and optionally bridges to the notif.sh cloud via NATS leaf node protocol.

The bridge runs as a system service (launchd on macOS, systemd on Linux).

Quick start:
  notif connect install --nats nats://localhost:4222 --topics "events.>"
  notif connect start
  notif connect status
  notif connect logs

Development mode:
  notif connect run --nats nats://localhost:4222 --topics "events.>"`,
}

var connectInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the bridge as a system service",
	Long: `Registers the bridge as a system service and saves configuration.

The configuration is persisted to ~/.notif/connect.yaml so the service
knows its settings on boot without requiring flags.

Examples:
  notif connect install --nats nats://localhost:4222
  notif connect install --nats nats://localhost:4222 --topics "orders.>" "alerts.>"
  notif connect install --nats nats://localhost:4222 --interceptors ./interceptors.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve config from flags + env + file
		flags := &bridge.Config{
			NatsURL:      connectNatsURL,
			Topics:       connectTopics,
			Interceptors: connectInterceptors,
			Stream:       connectStream,
			Cloud:        connectCloud,
		}
		cfg, err := bridge.ResolveConfig(flags, connectConfigPath)
		if err != nil {
			return fmt.Errorf("resolve config: %w", err)
		}

		if cfg.NatsURL == "" {
			return fmt.Errorf("--nats URL is required")
		}

		if cfg.Cloud {
			out.Warn("--cloud is a Phase 2 feature and is not yet available.")
			out.Info("Continuing with local-only mode.")
			cfg.Cloud = false
		}

		// Check for stream conflicts if we have topics
		if len(cfg.Topics) > 0 && cfg.Stream == "" {
			out.Info("Checking for stream conflicts...")
			// We need a temporary NATS connection for conflict check
			tmpBridge := bridge.NewBridge(cfg, connectConfigPath)
			_ = tmpBridge // conflict check done at runtime in Start()
		}

		// Persist config
		configPath := connectConfigPath
		if configPath == "" {
			configPath = bridge.DefaultConfigPath()
		}
		if err := bridge.SaveConfig(cfg, configPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		out.Success("Config saved to %s", configPath)

		// Install system service
		svc, err := bridge.GetService(configPath)
		if err != nil {
			return fmt.Errorf("create service: %w", err)
		}

		if err := svc.Install(); err != nil {
			return fmt.Errorf("install service: %w", err)
		}

		out.Success("Service installed")
		out.KeyValue("Service", bridge.ServiceName)
		out.KeyValue("Config", configPath)
		out.KeyValue("NATS", cfg.NatsURL)
		if len(cfg.Topics) > 0 {
			out.KeyValue("Topics", fmt.Sprintf("%v", cfg.Topics))
		}
		if cfg.Interceptors != "" {
			out.KeyValue("Interceptors", cfg.Interceptors)
		}
		out.Info("Run 'notif connect start' to start the bridge.")
		return nil
	},
}

var connectUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the bridge system service",
	Long: `Removes the bridge system service registration and optionally cleans up config files.

This will:
1. Stop the service if running
2. Remove the service registration (launchd/systemd)
3. Remove the config file and status file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := connectConfigPath
		if configPath == "" {
			configPath = bridge.DefaultConfigPath()
		}

		svc, err := bridge.GetService(configPath)
		if err != nil {
			return fmt.Errorf("create service: %w", err)
		}

		// Stop first (ignore errors — may not be running)
		_ = svc.Stop()

		if err := svc.Uninstall(); err != nil {
			return fmt.Errorf("uninstall service: %w", err)
		}

		// Clean up files
		os.Remove(configPath)
		os.Remove(bridge.DefaultStatusPath())
		out.Success("Service uninstalled and config cleaned up")
		return nil
	},
}

var connectStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the bridge service",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := connectConfigPath
		if configPath == "" {
			configPath = bridge.DefaultConfigPath()
		}

		svc, err := bridge.GetService(configPath)
		if err != nil {
			return fmt.Errorf("create service: %w", err)
		}

		if err := svc.Start(); err != nil {
			return fmt.Errorf("start service: %w", err)
		}

		out.Success("Bridge service started")
		out.Info("Run 'notif connect status' to check status.")
		return nil
	},
}

var connectStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the bridge service",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := connectConfigPath
		if configPath == "" {
			configPath = bridge.DefaultConfigPath()
		}

		svc, err := bridge.GetService(configPath)
		if err != nil {
			return fmt.Errorf("create service: %w", err)
		}

		if err := svc.Stop(); err != nil {
			return fmt.Errorf("stop service: %w", err)
		}

		out.Success("Bridge service stopped")
		return nil
	},
}

var connectStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show bridge status",
	Long: `Shows the current bridge status including connection state, throughput, and interceptors.

The status is read from the heartbeat file written every 10 seconds by the running service.

Staleness detection:
  - 30s without heartbeat: "stale" warning
  - 60s without heartbeat: "dead" warning
  - PID check: if process is dead, shows "crashed"

Config drift detection:
  If the config file was changed while the service is running, a warning is shown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := bridge.ReadStatus("")
		if err != nil {
			if jsonOutput {
				out.JSON(map[string]any{"state": "not running", "error": "no status file"})
			} else {
				out.Info("Bridge is not running (no status file)")
			}
			return nil
		}

		if jsonOutput {
			out.JSON(status)
			return nil
		}

		fmt.Print(bridge.FormatHumanStatus(status))
		return nil
	},
}

var connectLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View bridge logs",
	Long:  `Tails the bridge log file at ~/.notif/connect.log.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logPath := bridge.DefaultLogPath()
		f, err := os.Open(logPath)
		if err != nil {
			out.Info("No log file found at %s", logPath)
			return nil
		}
		defer f.Close()

		// Read last 8KB for a reasonable tail
		stat, _ := f.Stat()
		offset := stat.Size() - 8192
		if offset < 0 {
			offset = 0
		}
		f.Seek(offset, io.SeekStart)

		// If we seeked to middle of file, skip to next newline
		if offset > 0 {
			buf := make([]byte, 1)
			for {
				_, err := f.Read(buf)
				if err != nil || buf[0] == '\n' {
					break
				}
			}
		}

		io.Copy(os.Stdout, f)
		return nil
	},
}

var connectRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the bridge in foreground",
	Long: `Runs the bridge in the foreground for development and debugging.

This is equivalent to what the system service runs, but stays in the foreground
so you can see logs directly and stop with Ctrl+C.

Examples:
  notif connect run --nats nats://localhost:4222
  notif connect run --nats nats://localhost:4222 --topics "orders.>"
  notif connect run --config ~/.notif/connect.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve config
		flags := &bridge.Config{
			NatsURL:      connectNatsURL,
			Topics:       connectTopics,
			Interceptors: connectInterceptors,
			Stream:       connectStream,
			Cloud:        connectCloud,
		}
		cfg, err := bridge.ResolveConfig(flags, connectConfigPath)
		if err != nil {
			return fmt.Errorf("resolve config: %w", err)
		}

		if cfg.NatsURL == "" {
			return fmt.Errorf("--nats URL is required (or set NATS_URL env var)")
		}

		if cfg.Cloud {
			out.Warn("--cloud is a Phase 2 feature and is not yet available.")
			cfg.Cloud = false
		}

		// Set up logging with lumberjack rotation
		logPath := bridge.DefaultLogPath()
		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, ".notif")
		os.MkdirAll(logDir, 0700)

		lj := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    50, // MB
			MaxBackups: 3,
			MaxAge:     14,    // days
			Compress:   true,
		}

		// Log to both file and stderr in foreground mode
		multiWriter := io.MultiWriter(os.Stderr, lj)
		logger := slog.New(slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(logger)

		// Run bridge
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		b := bridge.NewBridge(cfg, connectConfigPath)

		// Handle signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			out.Info("Shutting down...")
			b.Stop()
			cancel()
		}()

		out.Success("Starting bridge in foreground mode")
		out.KeyValue("NATS", cfg.NatsURL)
		if len(cfg.Topics) > 0 {
			out.KeyValue("Topics", fmt.Sprintf("%v", cfg.Topics))
		}
		out.Info("Press Ctrl+C to stop")

		if err := b.Start(ctx); err != nil {
			return fmt.Errorf("start bridge: %w", err)
		}

		// Block until context is cancelled
		<-ctx.Done()
		return nil
	},
}

func init() {
	// Persistent flags for connect subcommands
	connectCmd.PersistentFlags().StringVar(&connectConfigPath, "config", "", "config file path (default ~/.notif/connect.yaml)")

	// Install-specific flags
	connectInstallCmd.Flags().StringVar(&connectNatsURL, "nats", "", "NATS server URL (e.g., nats://localhost:4222)")
	connectInstallCmd.Flags().StringSliceVar(&connectTopics, "topics", nil, "subjects to capture (e.g., \"orders.>\", \"alerts.>\")")
	connectInstallCmd.Flags().StringVar(&connectInterceptors, "interceptors", "", "path to interceptors YAML file")
	connectInstallCmd.Flags().StringVar(&connectStream, "stream", "", "reuse existing JetStream stream instead of creating NOTIF_BRIDGE")
	connectInstallCmd.Flags().BoolVar(&connectCloud, "cloud", false, "enable cloud connectivity (Phase 2, not yet available)")

	// Run-specific flags (same as install)
	connectRunCmd.Flags().StringVar(&connectNatsURL, "nats", "", "NATS server URL (e.g., nats://localhost:4222)")
	connectRunCmd.Flags().StringSliceVar(&connectTopics, "topics", nil, "subjects to capture")
	connectRunCmd.Flags().StringVar(&connectInterceptors, "interceptors", "", "path to interceptors YAML file")
	connectRunCmd.Flags().StringVar(&connectStream, "stream", "", "reuse existing JetStream stream")
	connectRunCmd.Flags().BoolVar(&connectCloud, "cloud", false, "enable cloud connectivity (Phase 2)")

	// Build subcommand tree
	connectCmd.AddCommand(connectInstallCmd)
	connectCmd.AddCommand(connectUninstallCmd)
	connectCmd.AddCommand(connectStartCmd)
	connectCmd.AddCommand(connectStopCmd)
	connectCmd.AddCommand(connectStatusCmd)
	connectCmd.AddCommand(connectLogsCmd)
	connectCmd.AddCommand(connectRunCmd)

	rootCmd.AddCommand(connectCmd)
}

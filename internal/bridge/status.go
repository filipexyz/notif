package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// BridgeState represents the bridge lifecycle state.
type BridgeState string

const (
	StateStarting BridgeState = "starting"
	StateRunning  BridgeState = "running"
	StateStopping BridgeState = "stopping"
	StateStopped  BridgeState = "stopped"
)

// StatusData is the JSON structure written to the status file.
type StatusData struct {
	State          BridgeState    `json:"state"`
	PID            int            `json:"pid"`
	StartedAt      time.Time      `json:"started_at"`
	LastHeartbeat  time.Time      `json:"last_heartbeat"`
	NatsURL        string         `json:"nats_url"`
	NatsConnected  bool           `json:"nats_connected"`
	Cloud          bool           `json:"cloud"`
	CloudConnected bool           `json:"cloud_connected,omitempty"`
	Interceptors   InterceptorStats `json:"interceptors"`
	Throughput     ThroughputStats  `json:"throughput"`
	Uptime         string         `json:"uptime"`
	ConfigDrift    []string       `json:"config_drift,omitempty"`
	StreamName     string         `json:"stream_name"`
	Topics         []string       `json:"topics,omitempty"`
}

// InterceptorStats tracks interceptor metrics.
type InterceptorStats struct {
	Active    int   `json:"active"`
	Processed int64 `json:"processed"`
	Errors    int64 `json:"errors"`
}

// ThroughputStats tracks message throughput.
type ThroughputStats struct {
	MsgsPerSec  float64 `json:"msgs_per_sec"`
	BytesPerSec float64 `json:"bytes_per_sec"`
}

// StatusReporter manages the status file and heartbeat.
type StatusReporter struct {
	statusPath string
	configPath string
	config     *Config
	startedAt  time.Time
	state      BridgeState

	mu            sync.RWMutex
	natsConnected bool
	interceptors  int
	processed     atomic.Int64
	errors        atomic.Int64
	msgBytes      atomic.Int64
	streamName    string
	topics        []string

	// throughput tracking
	prevProcessed int64
	prevBytes     int64
	prevTime      time.Time
	msgsPerSec    float64
	bytesPerSec   float64
}

// NewStatusReporter creates a new StatusReporter.
func NewStatusReporter(statusPath, configPath string, config *Config) *StatusReporter {
	if statusPath == "" {
		statusPath = DefaultStatusPath()
	}
	if configPath == "" {
		configPath = DefaultConfigPath()
	}
	return &StatusReporter{
		statusPath: statusPath,
		configPath: configPath,
		config:     config,
		startedAt:  time.Now(),
		state:      StateStarting,
		prevTime:   time.Now(),
	}
}

// SetState updates the bridge state.
func (sr *StatusReporter) SetState(s BridgeState) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.state = s
}

// SetNatsConnected updates the NATS connection status.
func (sr *StatusReporter) SetNatsConnected(connected bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.natsConnected = connected
}

// SetInterceptorCount updates the active interceptor count.
func (sr *StatusReporter) SetInterceptorCount(n int) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.interceptors = n
}

// SetStreamInfo updates stream name and topics.
func (sr *StatusReporter) SetStreamInfo(name string, topics []string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.streamName = name
	sr.topics = topics
}

// RecordProcessed increments the processed counter.
func (sr *StatusReporter) RecordProcessed(bytes int) {
	sr.processed.Add(1)
	sr.msgBytes.Add(int64(bytes))
}

// RecordError increments the error counter.
func (sr *StatusReporter) RecordError() {
	sr.errors.Add(1)
}

// WriteHeartbeat writes current status to the status file.
func (sr *StatusReporter) WriteHeartbeat() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	now := time.Now()

	// Calculate throughput
	elapsed := now.Sub(sr.prevTime).Seconds()
	if elapsed > 0 {
		currentProcessed := sr.processed.Load()
		currentBytes := sr.msgBytes.Load()
		sr.msgsPerSec = float64(currentProcessed-sr.prevProcessed) / elapsed
		sr.bytesPerSec = float64(currentBytes-sr.prevBytes) / elapsed
		sr.prevProcessed = currentProcessed
		sr.prevBytes = currentBytes
		sr.prevTime = now
	}

	// Detect config drift
	drift := DetectDrift(sr.config, sr.configPath)

	data := StatusData{
		State:         sr.state,
		PID:           os.Getpid(),
		StartedAt:     sr.startedAt,
		LastHeartbeat: now,
		NatsURL:       sr.config.NatsURL,
		NatsConnected: sr.natsConnected,
		Cloud:         sr.config.Cloud,
		Interceptors: InterceptorStats{
			Active:    sr.interceptors,
			Processed: sr.processed.Load(),
			Errors:    sr.errors.Load(),
		},
		Throughput: ThroughputStats{
			MsgsPerSec:  sr.msgsPerSec,
			BytesPerSec: sr.bytesPerSec,
		},
		Uptime:      formatDuration(now.Sub(sr.startedAt)),
		ConfigDrift: drift,
		StreamName:  sr.streamName,
		Topics:      sr.topics,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	_ = os.MkdirAll(filepath.Dir(sr.statusPath), 0700)
	return os.WriteFile(sr.statusPath, jsonData, 0644)
}

// Cleanup removes the status file.
func (sr *StatusReporter) Cleanup() {
	os.Remove(sr.statusPath)
}

// ReadStatus reads the status file from disk.
func ReadStatus(path string) (*StatusData, error) {
	if path == "" {
		path = DefaultStatusPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read status: %w", err)
	}
	var status StatusData
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &status, nil
}

// FormatHumanStatus formats status data for human-readable display.
func FormatHumanStatus(s *StatusData) string {
	var b strings.Builder

	// Check staleness
	age := time.Since(s.LastHeartbeat)
	stateStr := string(s.State)

	// PID-based crash detection
	if s.State == StateRunning && !isProcessAlive(s.PID) {
		stateStr = "crashed (process dead)"
	} else if age > 60*time.Second {
		stateStr = fmt.Sprintf("dead (no heartbeat for %s)", formatDuration(age))
	} else if age > 30*time.Second {
		stateStr = fmt.Sprintf("stale (no heartbeat for %s)", formatDuration(age))
	}

	fmt.Fprintf(&b, "Bridge:        %s (uptime: %s)\n", stateStr, s.Uptime)
	fmt.Fprintf(&b, "PID:           %d\n", s.PID)

	connStatus := "disconnected"
	if s.NatsConnected {
		connStatus = "connected"
	}
	fmt.Fprintf(&b, "Local NATS:    %s (%s)\n", connStatus, s.NatsURL)

	if s.Cloud {
		cloudStatus := "disconnected"
		if s.CloudConnected {
			cloudStatus = "connected"
		}
		fmt.Fprintf(&b, "Cloud:         %s\n", cloudStatus)
	} else {
		fmt.Fprintf(&b, "Cloud:         disabled (Phase 2)\n")
	}

	fmt.Fprintf(&b, "Stream:        %s\n", s.StreamName)
	if len(s.Topics) > 0 {
		fmt.Fprintf(&b, "Topics:        %s\n", strings.Join(s.Topics, ", "))
	}

	fmt.Fprintf(&b, "Interceptors:  %d active (%d processed, %d errors)\n",
		s.Interceptors.Active, s.Interceptors.Processed, s.Interceptors.Errors)
	fmt.Fprintf(&b, "Throughput:    %.1f msgs/s (%.1f KB/s)\n",
		s.Throughput.MsgsPerSec, s.Throughput.BytesPerSec/1024)
	fmt.Fprintf(&b, "Heartbeat:     %s ago\n", formatDuration(age))

	if len(s.ConfigDrift) > 0 {
		fmt.Fprintf(&b, "\nConfig drift detected: %s changed\n", strings.Join(s.ConfigDrift, ", "))
		fmt.Fprintf(&b, "  Run `notif connect stop && notif connect start` to apply.\n")
	}

	return b.String()
}

// isProcessAlive checks if a PID is still running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without actually sending a signal
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	d = d.Round(time.Second)

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

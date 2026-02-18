package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubjectsOverlap(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []string
		overlap bool
	}{
		{"exact match", []string{"orders.created"}, []string{"orders.created"}, true},
		{"no overlap", []string{"orders.>"}, []string{"alerts.>"}, false},
		{"wildcard overlap", []string{"orders.>"}, []string{"orders.created"}, true},
		{"star overlap", []string{"orders.*"}, []string{"orders.created"}, true},
		{"star no overlap depth", []string{"orders.*"}, []string{"orders.created.v2"}, false},
		{"gt vs star", []string{"orders.>"}, []string{"orders.*"}, true},
		{"disjoint", []string{"a.b.c"}, []string{"x.y.z"}, false},
		{"multiple subjects", []string{"a.>", "b.>"}, []string{"b.test"}, true},
		{"empty a", []string{}, []string{"b.>"}, false},
		{"both gt", []string{">"}, []string{">"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subjectsOverlap(tt.a, tt.b)
			if got != tt.overlap {
				t.Errorf("subjectsOverlap(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.overlap)
			}
		})
	}
}

func TestNatsSubjectOverlaps(t *testing.T) {
	tests := []struct {
		a, b    string
		overlap bool
	}{
		{"foo.bar", "foo.bar", true},
		{"foo.bar", "foo.baz", false},
		{"foo.*", "foo.bar", true},
		{"foo.*", "foo.bar.baz", false},
		{"foo.>", "foo.bar", true},
		{"foo.>", "foo.bar.baz", true},
		{"*.*", "foo.bar", true},
		{"*.>", "foo.bar", true},
		{">", "anything.here", true},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := natsSubjectOverlaps(tt.a, tt.b)
			if got != tt.overlap {
				t.Errorf("natsSubjectOverlaps(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.overlap)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connect.yaml")

	cfg := &Config{
		NatsURL:      "nats://localhost:4222",
		Topics:       []string{"orders.>", "alerts.>"},
		Interceptors: "/path/to/interceptors.yaml",
		Stream:       "",
		Cloud:        false,
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if loaded.NatsURL != cfg.NatsURL {
		t.Errorf("NatsURL = %q, want %q", loaded.NatsURL, cfg.NatsURL)
	}
	if len(loaded.Topics) != len(cfg.Topics) {
		t.Errorf("Topics length = %d, want %d", len(loaded.Topics), len(cfg.Topics))
	}
	if loaded.Interceptors != cfg.Interceptors {
		t.Errorf("Interceptors = %q, want %q", loaded.Interceptors, cfg.Interceptors)
	}
}

func TestResolveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connect.yaml")

	fileCfg := &Config{
		NatsURL: "nats://file:4222",
		Topics:  []string{"file.>"},
	}
	if err := SaveConfig(fileCfg, path); err != nil {
		t.Fatal(err)
	}

	flags := &Config{
		NatsURL: "nats://flag:4222",
	}
	resolved, err := ResolveConfig(flags, path)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.NatsURL != "nats://flag:4222" {
		t.Errorf("NatsURL = %q, want flag override", resolved.NatsURL)
	}
	if len(resolved.Topics) != 1 || resolved.Topics[0] != "file.>" {
		t.Errorf("Topics = %v, want file topics", resolved.Topics)
	}
}

func TestResolveConfigEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connect.yaml")

	fileCfg := &Config{NatsURL: "nats://file:4222"}
	SaveConfig(fileCfg, path)

	os.Setenv("NATS_URL", "nats://env:4222")
	defer os.Unsetenv("NATS_URL")

	flags := &Config{}
	resolved, _ := ResolveConfig(flags, path)

	if resolved.NatsURL != "nats://env:4222" {
		t.Errorf("NatsURL = %q, want env override", resolved.NatsURL)
	}
}

func TestDetectDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connect.yaml")

	fileCfg := &Config{NatsURL: "nats://localhost:4222", Topics: []string{"a.>"}}
	SaveConfig(fileCfg, path)

	running := &Config{NatsURL: "nats://localhost:4223", Topics: []string{"a.>"}}
	drift := DetectDrift(running, path)

	if len(drift) == 0 {
		t.Error("expected drift detection for NatsURL change")
	}
	found := false
	for _, d := range drift {
		if d == "nats_url" {
			found = true
		}
	}
	if !found {
		t.Errorf("drift = %v, expected nats_url", drift)
	}

	same := &Config{NatsURL: "nats://localhost:4222", Topics: []string{"a.>"}}
	noDrift := DetectDrift(same, path)
	if len(noDrift) != 0 {
		t.Errorf("expected no drift, got %v", noDrift)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "0s"},
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m5s"},
		{3661 * time.Second, "1h1m"},
		{86500 * time.Second, "1d0h1m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatHumanStatus(t *testing.T) {
	s := &StatusData{
		State:         StateRunning,
		PID:           os.Getpid(),
		NatsURL:       "nats://localhost:4222",
		NatsConnected: true,
		StreamName:    "NOTIF_BRIDGE",
		Topics:        []string{"orders.>"},
		Interceptors:  InterceptorStats{Active: 2, Processed: 100, Errors: 1},
		Throughput:    ThroughputStats{MsgsPerSec: 10.5, BytesPerSec: 1024},
		Uptime:        "1h5m",
	}

	output := FormatHumanStatus(s)
	if output == "" {
		t.Error("expected non-empty human status output")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"my-interceptor", "my-interceptor"},
		{"transform orders", "transform-orders"},
		{"a.b.c", "a-b-c"},
		{"test_123", "test_123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadInterceptors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "interceptors.yaml")

	content := `interceptors:
  - name: transform-orders
    from: "orders.>"
    to: "processed.orders"
    jq: ".data"
  - name: filter-alerts
    from: "alerts.>"
    to: "filtered.alerts"
    jq: "select(.severity == \"high\")"
`
	os.WriteFile(path, []byte(content), 0644)

	configs, err := LoadInterceptors(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 interceptors, got %d", len(configs))
	}
	if configs[0].Name != "transform-orders" {
		t.Errorf("first interceptor name = %q", configs[0].Name)
	}
	if configs[1].JQ != `select(.severity == "high")` {
		t.Errorf("second interceptor jq = %q", configs[1].JQ)
	}
}

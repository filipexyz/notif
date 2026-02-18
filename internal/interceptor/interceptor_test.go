package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// testEnv provides an embedded NATS server + JetStream for testing.
type testEnv struct {
	srv    *server.Server
	nc     *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random port
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("create nats server: %v", err)
	}
	srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("connect to nats: %v", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		srv.Shutdown()
		t.Fatalf("create jetstream: %v", err)
	}

	stream, err := js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "NOTIF_EVENTS",
		Subjects: []string{"events.>"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		nc.Close()
		srv.Shutdown()
		t.Fatalf("create stream: %v", err)
	}

	t.Cleanup(func() {
		nc.Close()
		srv.Shutdown()
	})

	return &testEnv{srv: srv, nc: nc, js: js, stream: stream}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// waitForMessage subscribes to a subject and waits for one message with a timeout.
func waitForMessage(t *testing.T, env *testEnv, subject string, timeout time.Duration) jetstream.Msg {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create an ephemeral consumer for the target subject
	cons, err := env.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubjects: []string{subject},
		AckPolicy:      jetstream.AckExplicitPolicy,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
	})
	if err != nil {
		t.Fatalf("create test consumer: %v", err)
	}

	msg, err := cons.Next(jetstream.FetchMaxWait(timeout))
	if err != nil {
		t.Fatalf("waiting for message on %s: %v", subject, err)
	}
	_ = msg.Ack()
	return msg
}

// Test 1: Interceptor subscribes and republishes to target subject
func TestInterceptor_BasicForward(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	intc, err := New("test-fwd", "events.org.proj.inbound.>", "events.org.proj.output.>", "", env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start interceptor: %v", err)
	}
	defer intc.Stop()

	// Allow consumer to be ready
	time.Sleep(200 * time.Millisecond)

	payload := map[string]string{"hello": "world"}
	data, _ := json.Marshal(payload)
	if _, err := env.js.Publish(ctx, "events.org.proj.inbound.chat", data); err != nil {
		t.Fatalf("publish test message: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.output.>", 5*time.Second)

	if msg.Subject() != "events.org.proj.output.chat" {
		t.Errorf("expected subject events.org.proj.output.chat, got %s", msg.Subject())
	}

	var result map[string]string
	if err := json.Unmarshal(msg.Data(), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", result)
	}
}

// Test 2: jq transform reshapes payload correctly
func TestInterceptor_JqTransform(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	jqExpr := `{text: .textContent, sender: .senderDisplayName}`
	intc, err := New("test-jq", "events.org.proj.inbound.>", "events.org.proj.transformed.>", jqExpr, env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start interceptor: %v", err)
	}
	defer intc.Stop()

	time.Sleep(200 * time.Millisecond)

	payload := map[string]interface{}{
		"textContent":       "Hello there",
		"senderDisplayName": "Alice",
		"extraField":        "ignored",
	}
	data, _ := json.Marshal(payload)
	if _, err := env.js.Publish(ctx, "events.org.proj.inbound.msg", data); err != nil {
		t.Fatalf("publish test message: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.transformed.>", 5*time.Second)

	var result map[string]interface{}
	if err := json.Unmarshal(msg.Data(), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["text"] != "Hello there" {
		t.Errorf("expected text=Hello there, got %v", result["text"])
	}
	if result["sender"] != "Alice" {
		t.Errorf("expected sender=Alice, got %v", result["sender"])
	}
	if _, exists := result["extraField"]; exists {
		t.Error("extraField should have been removed by jq transform")
	}
}

// Test 3: jq select filter drops non-matching events
func TestInterceptor_JqSelectFilter(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	jqExpr := `select(.status == "active") | {name: .name}`
	intc, err := New("test-select", "events.org.proj.inbound.>", "events.org.proj.filtered.>", jqExpr, env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start interceptor: %v", err)
	}
	defer intc.Stop()

	time.Sleep(200 * time.Millisecond)

	// This one should be filtered out (status != active)
	dropped := map[string]interface{}{"name": "Bob", "status": "inactive"}
	data1, _ := json.Marshal(dropped)
	if _, err := env.js.Publish(ctx, "events.org.proj.inbound.user", data1); err != nil {
		t.Fatalf("publish dropped message: %v", err)
	}

	// This one should pass through
	passed := map[string]interface{}{"name": "Alice", "status": "active"}
	data2, _ := json.Marshal(passed)
	if _, err := env.js.Publish(ctx, "events.org.proj.inbound.user", data2); err != nil {
		t.Fatalf("publish passed message: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.filtered.>", 5*time.Second)

	var result map[string]interface{}
	if err := json.Unmarshal(msg.Data(), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
}

// Test 4: Passthrough mode (no jq) forwards data unchanged
func TestInterceptor_Passthrough(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	intc, err := New("test-pass", "events.org.proj.src.>", "events.org.proj.dst.>", "", env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start interceptor: %v", err)
	}
	defer intc.Stop()

	time.Sleep(200 * time.Millisecond)

	original := `{"nested":{"deep":true},"list":[1,2,3]}`
	if _, err := env.js.Publish(ctx, "events.org.proj.src.data", []byte(original)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.dst.>", 5*time.Second)

	if string(msg.Data()) != original {
		t.Errorf("expected passthrough data %q, got %q", original, string(msg.Data()))
	}
}

// Test 5: Loop prevention (message with interceptor header is skipped)
func TestInterceptor_LoopPrevention(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	// This interceptor reads from src and writes to dst
	intc, err := New("test-loop", "events.org.proj.src.>", "events.org.proj.dst.>", "", env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start interceptor: %v", err)
	}
	defer intc.Stop()

	time.Sleep(200 * time.Millisecond)

	// Publish a message WITH the interceptor header already set -> should be skipped
	outMsg := &nats.Msg{
		Subject: "events.org.proj.src.looped",
		Data:    []byte(`{"skip":"me"}`),
		Header:  nats.Header{},
	}
	outMsg.Header.Set(headerKey, "test-loop")
	if _, err := env.js.PublishMsg(ctx, outMsg); err != nil {
		t.Fatalf("publish looped message: %v", err)
	}

	// Now publish a normal message that should go through
	if _, err := env.js.Publish(ctx, "events.org.proj.src.normal", []byte(`{"pass":"through"}`)); err != nil {
		t.Fatalf("publish normal message: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.dst.>", 5*time.Second)

	// The received message should be the normal one, not the looped one
	var result map[string]string
	if err := json.Unmarshal(msg.Data(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["pass"] != "through" {
		t.Errorf("expected the non-looped message, got %v", result)
	}
}

// Test 6: Subject mapping (from prefix replaced with to prefix)
func TestInterceptor_SubjectMapping(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		subject string
		want    string
	}{
		{
			name:    "simple wildcard",
			from:    "events.org.proj.inbound.>",
			to:      "events.org.proj.outbound.>",
			subject: "events.org.proj.inbound.chat",
			want:    "events.org.proj.outbound.chat",
		},
		{
			name:    "deep wildcard",
			from:    "events.org.proj.inbound.>",
			to:      "events.org.proj.outbound.>",
			subject: "events.org.proj.inbound.chat.messages",
			want:    "events.org.proj.outbound.chat.messages",
		},
		{
			name:    "star wildcard",
			from:    "events.org.proj.inbound.*",
			to:      "events.org.proj.outbound.*",
			subject: "events.org.proj.inbound.chat",
			want:    "events.org.proj.outbound.chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intc := &Interceptor{from: tt.from, to: tt.to}
			got := intc.mapSubject(tt.subject)
			if got != tt.want {
				t.Errorf("mapSubject(%q) = %q, want %q", tt.subject, got, tt.want)
			}
		})
	}
}

// Test 7: Start/Stop lifecycle
func TestInterceptor_StartStop(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	intc, err := New("test-lifecycle", "events.org.proj.life.>", "events.org.proj.dest.>", "", env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		intc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}

// Test: Config loading
func TestLoadConfig(t *testing.T) {
	content := `
interceptors:
  - name: reshape-inbound
    from: "events.org_default.default.omni.inbound.>"
    to: "events.org_default.default.omni.transformed.>"
    jq: '{text: .textContent, sender: .senderDisplayName}'
    enabled: true
  - name: disabled-one
    from: "events.a.>"
    to: "events.b.>"
    enabled: false
  - name: defaults-enabled
    from: "events.c.>"
    to: "events.d.>"
`
	tmpFile := t.TempDir() + "/interceptors.yaml"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Interceptors) != 3 {
		t.Fatalf("expected 3 interceptors, got %d", len(cfg.Interceptors))
	}

	// First: enabled explicitly
	if !cfg.Interceptors[0].IsEnabled() {
		t.Error("first interceptor should be enabled")
	}
	if cfg.Interceptors[0].Name != "reshape-inbound" {
		t.Errorf("expected name reshape-inbound, got %s", cfg.Interceptors[0].Name)
	}
	if cfg.Interceptors[0].Jq != "{text: .textContent, sender: .senderDisplayName}" {
		t.Errorf("jq expression mismatch: %s", cfg.Interceptors[0].Jq)
	}

	// Second: explicitly disabled
	if cfg.Interceptors[1].IsEnabled() {
		t.Error("second interceptor should be disabled")
	}

	// Third: defaults to enabled
	if !cfg.Interceptors[2].IsEnabled() {
		t.Error("third interceptor should default to enabled")
	}
}

// Test: Manager creates only enabled interceptors
func TestManager_SkipsDisabled(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	f := false
	cfg := &Config{
		Interceptors: []InterceptorConfig{
			{Name: "enabled", From: "events.a.>", To: "events.b.>"},
			{Name: "disabled", From: "events.c.>", To: "events.d.>", Enabled: &f},
		},
	}

	mgr, err := NewManager(cfg, env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	if len(mgr.interceptors) != 1 {
		t.Errorf("expected 1 interceptor, got %d", len(mgr.interceptors))
	}
	if mgr.interceptors[0].name != "enabled" {
		t.Errorf("expected interceptor name 'enabled', got %s", mgr.interceptors[0].name)
	}
}

// Test: Manager Start/Stop lifecycle
func TestManager_StartStop(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	cfg := &Config{
		Interceptors: []InterceptorConfig{
			{Name: "m1", From: "events.org.proj.a.>", To: "events.org.proj.b.>"},
			{Name: "m2", From: "events.org.proj.c.>", To: "events.org.proj.d.>"},
		},
	}

	mgr, err := NewManager(cfg, env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("start manager: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Publish to first interceptor's source
	if _, err := env.js.Publish(ctx, "events.org.proj.a.test", []byte(`{"m":"1"}`)); err != nil {
		t.Fatalf("publish to m1: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.b.>", 5*time.Second)
	if msg.Subject() != "events.org.proj.b.test" {
		t.Errorf("expected events.org.proj.b.test, got %s", msg.Subject())
	}

	// Stop should not hang
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mgr.Stop()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("manager Stop() hung")
	}
}

// Test: staticPrefix helper
func TestStaticPrefix(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"events.org.proj.inbound.>", "events.org.proj.inbound."},
		{"events.org.proj.inbound.*", "events.org.proj.inbound."},
		{"events.>", "events."},
		{">", ""},
		{"*", ""},
		{"events.org.proj.exact", "events.org.proj.exact."},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("staticPrefix(%s)", tt.pattern), func(t *testing.T) {
			got := staticPrefix(tt.pattern)
			if got != tt.want {
				t.Errorf("staticPrefix(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

// Test: Output message has the loop prevention header
func TestInterceptor_OutputHasHeader(t *testing.T) {
	env := setupTestEnv(t)
	logger := testLogger()

	intc, err := New("test-hdr", "events.org.proj.hdr.>", "events.org.proj.hdrout.>", "", env.js, env.stream, logger)
	if err != nil {
		t.Fatalf("create interceptor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := intc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer intc.Stop()

	time.Sleep(200 * time.Millisecond)

	if _, err := env.js.Publish(ctx, "events.org.proj.hdr.test", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg := waitForMessage(t, env, "events.org.proj.hdrout.>", 5*time.Second)

	hdrs := msg.Headers()
	if hdrs == nil {
		t.Fatal("expected headers on output message")
	}
	val := hdrs.Get(headerKey)
	if val != "test-hdr" {
		t.Errorf("expected header %s=test-hdr, got %q", headerKey, val)
	}
}

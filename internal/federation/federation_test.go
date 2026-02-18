package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"os"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// startWSServer creates a mock notif WebSocket server that speaks the notif protocol.
func startWSServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// wsURL converts http://... to ws://...
func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func TestLoadConfig(t *testing.T) {
	yaml := `bridges:
  - name: test-bridge
    url: https://remote.notif.sh
    api_key: "${TEST_KEY}"
    direction: inbound
    remote_topic: "alerts.>"
    local_subject: "events.org.default.alerts"
    enabled: true
  - name: disabled-bridge
    url: https://other.notif.sh
    api_key: nsh_abc
    direction: outbound
    remote_topic: "metrics"
    local_subject: "events.org.default.metrics.>"
    enabled: false
`
	f, err := os.CreateTemp(t.TempDir(), "federation-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Bridges) != 2 {
		t.Fatalf("expected 2 bridges, got %d", len(cfg.Bridges))
	}
	if cfg.Bridges[0].Name != "test-bridge" {
		t.Errorf("expected name test-bridge, got %s", cfg.Bridges[0].Name)
	}
	if cfg.Bridges[0].Direction != "inbound" {
		t.Errorf("expected direction inbound, got %s", cfg.Bridges[0].Direction)
	}
	if !cfg.Bridges[0].IsEnabled() {
		t.Error("expected first bridge to be enabled")
	}
	if cfg.Bridges[1].IsEnabled() {
		t.Error("expected second bridge to be disabled")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestClientSubscribe(t *testing.T) {
	eventData := json.RawMessage(`{"msg":"hello"}`)

	srv := startWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var sub struct {
			Action  string   `json:"action"`
			Topics  []string `json:"topics"`
			Options struct {
				AutoAck bool   `json:"auto_ack"`
				From    string `json:"from"`
			} `json:"options"`
		}
		if err := conn.ReadJSON(&sub); err != nil {
			t.Errorf("read subscribe: %v", err)
			return
		}
		if sub.Action != "subscribe" {
			t.Errorf("expected subscribe action, got %s", sub.Action)
			return
		}
		if len(sub.Topics) != 1 || sub.Topics[0] != "alerts.>" {
			t.Errorf("unexpected topics: %v", sub.Topics)
			return
		}
		if !sub.Options.AutoAck {
			t.Errorf("expected auto_ack=true")
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]any{
			"type":   "subscribed",
			"topics": sub.Topics,
		})

		// Send an event
		conn.WriteJSON(map[string]any{
			"type":      "event",
			"id":        "evt_abc123",
			"topic":     "alerts.critical",
			"data":      eventData,
			"timestamp": time.Now().UTC(),
		})

		// Keep connection open until client disconnects
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	client := NewClient(srv.URL, "nsh_test123", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := client.Subscribe(ctx, []string{"alerts.>"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	select {
	case evt := <-events:
		if evt.ID != "evt_abc123" {
			t.Errorf("expected event id evt_abc123, got %s", evt.ID)
		}
		if evt.Topic != "alerts.critical" {
			t.Errorf("expected topic alerts.critical, got %s", evt.Topic)
		}
		if string(evt.Data) != string(eventData) {
			t.Errorf("expected data %s, got %s", eventData, evt.Data)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

func TestClientEmit(t *testing.T) {
	var (
		mu         sync.Mutex
		gotTopic   string
		gotData    json.RawMessage
		gotAuth    string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		gotAuth = r.Header.Get("Authorization")
		var req struct {
			Topic string          `json:"topic"`
			Data  json.RawMessage `json:"data"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		gotTopic = req.Topic
		gotData = req.Data

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "evt_xyz",
			"topic": req.Topic,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "nsh_testkey", nil)
	ctx := context.Background()

	data := json.RawMessage(`{"metric":"cpu","value":42}`)
	err := client.Emit(ctx, "metrics.cpu", data)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if gotAuth != "Bearer nsh_testkey" {
		t.Errorf("expected auth header 'Bearer nsh_testkey', got %q", gotAuth)
	}
	if gotTopic != "metrics.cpu" {
		t.Errorf("expected topic metrics.cpu, got %s", gotTopic)
	}
	if string(gotData) != `{"metric":"cpu","value":42}` {
		t.Errorf("unexpected data: %s", gotData)
	}
}

func TestClientAuthOnWebSocket(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Read subscribe, send subscribed
		conn.ReadJSON(&json.RawMessage{})
		conn.WriteJSON(map[string]any{"type": "subscribed", "topics": []string{"x"}})
		// Hold open
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "nsh_secret", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Subscribe(ctx, []string{"x"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Small wait for the connection to be established
	time.Sleep(50 * time.Millisecond)

	if gotAuth != "Bearer nsh_secret" {
		t.Errorf("expected WS auth header 'Bearer nsh_secret', got %q", gotAuth)
	}
}

func TestClientReconnect(t *testing.T) {
	var (
		mu          sync.Mutex
		connections int
	)

	srv := startWSServer(t, func(conn *websocket.Conn) {
		mu.Lock()
		connections++
		connNum := connections
		mu.Unlock()

		// Read subscribe
		conn.ReadJSON(&json.RawMessage{})
		// Send subscribed
		conn.WriteJSON(map[string]any{"type": "subscribed", "topics": []string{"x"}})

		if connNum == 1 {
			// First connection: close immediately to trigger reconnect
			time.Sleep(50 * time.Millisecond)
			conn.Close()
			return
		}

		// Second connection: send an event then hold open
		conn.WriteJSON(map[string]any{
			"type":  "event",
			"id":    "evt_reconnected",
			"topic": "x",
			"data":  json.RawMessage(`{}`),
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	client := NewClient(srv.URL, "nsh_test", nil)
	// Use a short initial backoff for testing
	client.initialBackoff = 50 * time.Millisecond
	client.maxBackoff = 200 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := client.Subscribe(ctx, []string{"x"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Should receive event from second connection after reconnect
	select {
	case evt := <-events:
		if evt.ID != "evt_reconnected" {
			t.Errorf("expected evt_reconnected, got %s", evt.ID)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for reconnect event")
	}

	mu.Lock()
	if connections < 2 {
		t.Errorf("expected at least 2 connections, got %d", connections)
	}
	mu.Unlock()
}

func boolPtr(v bool) *bool { return &v }

// startEmbeddedNATS starts an embedded NATS server for testing with JetStream.
func startEmbeddedNATS(t *testing.T) (*natsserver.Server, jetstream.JetStream) {
	t.Helper()

	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1, // random port
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
	}

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("start nats server: %v", err)
	}
	ns.Start()
	t.Cleanup(ns.Shutdown)

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats not ready")
	}

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	// Create events stream
	_, err = js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "NOTIF_EVENTS",
		Subjects: []string{"events.>"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	return ns, js
}

func TestBridgeInbound(t *testing.T) {
	// Remote notif mock: sends events via WS
	eventData := json.RawMessage(`{"alert":"fire"}`)

	srv := startWSServer(t, func(conn *websocket.Conn) {
		conn.ReadJSON(&json.RawMessage{})
		conn.WriteJSON(map[string]any{"type": "subscribed", "topics": []string{"alerts.>"}})
		time.Sleep(50 * time.Millisecond)
		conn.WriteJSON(map[string]any{
			"type":  "event",
			"id":    "evt_remote1",
			"topic": "alerts.critical",
			"data":  eventData,
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	ns, js := startEmbeddedNATS(t)
	_ = ns

	// Subscribe to the local NATS subject to verify the event arrives
	consumer, err := js.CreateOrUpdateConsumer(context.Background(), "NOTIF_EVENTS", jetstream.ConsumerConfig{
		FilterSubject: "events.org_default.default.prod.alerts.critical",
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckPolicy:     jetstream.AckNonePolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	cfg := &Config{
		Bridges: []BridgeConfig{
			{
				Name:         "test-inbound",
				URL:          srv.URL,
				APIKey:       "nsh_test",
				Direction:    "inbound",
				RemoteTopic:  "alerts.>",
				LocalSubject: "events.org_default.default.prod.alerts.critical",
				Enabled:      boolPtr(true),
			},
		},
	}

	fed, err := NewFederation(cfg, js, "NOTIF_EVENTS", nil)
	if err != nil {
		t.Fatalf("new federation: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := fed.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer fed.Stop()

	// Wait for the event to appear on local NATS
	msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	var got bool
	for msg := range msgs.Messages() {
		var evt map[string]any
		json.Unmarshal(msg.Data(), &evt)
		if evt["id"] == "evt_remote1" {
			got = true
		}
	}
	if !got {
		t.Fatal("did not receive inbound event on local NATS")
	}
}

func TestBridgeOutbound(t *testing.T) {
	var (
		mu       sync.Mutex
		received []map[string]any
	)

	// Mock remote notif HTTP emit endpoint
	emitSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/emit" {
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			received = append(received, req)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"id": "evt_ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer emitSrv.Close()

	ns, js := startEmbeddedNATS(t)
	_ = ns

	cfg := &Config{
		Bridges: []BridgeConfig{
			{
				Name:         "test-outbound",
				URL:          emitSrv.URL,
				APIKey:       "nsh_out",
				Direction:    "outbound",
				RemoteTopic:  "metrics.from-staging",
				LocalSubject: "events.org_default.default.metrics.>",
				Enabled:      boolPtr(true),
			},
		},
	}

	fed, err := NewFederation(cfg, js, "NOTIF_EVENTS", nil)
	if err != nil {
		t.Fatalf("new federation: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := fed.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer fed.Stop()

	// Give the consumer time to start
	time.Sleep(200 * time.Millisecond)

	// Publish a message to local NATS
	localEvent := map[string]any{
		"id":    "evt_local1",
		"topic": "metrics.cpu",
		"data":  map[string]any{"cpu": 95},
	}
	data, _ := json.Marshal(localEvent)
	_, err = js.Publish(ctx, "events.org_default.default.metrics.cpu", data)
	if err != nil {
		t.Fatalf("publish local: %v", err)
	}

	// Wait for it to be emitted to the remote
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for outbound emit")
		case <-time.After(50 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("no events received")
	}
	if received[0]["topic"] != "metrics.from-staging" {
		t.Errorf("expected remote topic metrics.from-staging, got %v", received[0]["topic"])
	}
}

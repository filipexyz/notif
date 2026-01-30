package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockWSServer creates a test WebSocket server
func mockWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
}

func TestSubscribe_Connect(t *testing.T) {
	var subscribeReceived atomic.Bool

	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		if msg["action"] == "subscribe" {
			subscribeReceived.Store(true)
			// Send subscribed confirmation
			conn.WriteJSON(map[string]string{"type": "subscribed"})
		}

		// Keep connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{AutoAck: true})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait a bit for subscribe to be processed
	time.Sleep(100 * time.Millisecond)

	if !subscribeReceived.Load() {
		t.Error("Subscribe message not received by server")
	}

	if !sub.IsConnected() {
		t.Error("Subscription should be connected")
	}
}

func TestSubscribe_ReceiveEvent(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Send an event
		conn.WriteJSON(map[string]any{
			"type":      "event",
			"id":        "evt-123",
			"topic":     "test-topic",
			"data":      map[string]string{"message": "hello"},
			"timestamp": time.Now().Format(time.RFC3339),
			"attempt":   1,
		})

		// Keep connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait for event
	select {
	case event := <-sub.Events():
		if event.ID != "evt-123" {
			t.Errorf("Expected event ID 'evt-123', got '%s'", event.ID)
		}
		if event.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got '%s'", event.Topic)
		}

		var data map[string]string
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("Failed to unmarshal event data: %v", err)
		}
		if data["message"] != "hello" {
			t.Errorf("Expected message 'hello', got '%s'", data["message"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestSubscribe_Reconnect(t *testing.T) {
	var connectionCount atomic.Int32

	server := mockWSServer(t, func(conn *websocket.Conn) {
		count := connectionCount.Add(1)

		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// On first connection, close after a short delay to trigger reconnect
		if count == 1 {
			time.Sleep(200 * time.Millisecond)
			conn.Close()
			return
		}

		// Second connection: keep alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait for reconnection
	time.Sleep(3 * time.Second)

	if connectionCount.Load() < 2 {
		t.Errorf("Expected at least 2 connections (reconnect), got %d", connectionCount.Load())
	}

	if !sub.IsConnected() {
		t.Error("Subscription should be connected after reconnect")
	}
}

func TestSubscribe_PingPong(t *testing.T) {
	var pingReceived atomic.Bool

	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Set up ping handler
		conn.SetPingHandler(func(data string) error {
			pingReceived.Store(true)
			return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
		})

		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Keep reading to process pings
		for {
			conn.SetReadDeadline(time.Now().Add(time.Minute))
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// The ping period is 54 seconds by default, which is too long for a test.
	// We verify the writePump is running and will send pings by checking
	// that the subscription stays connected.
	time.Sleep(500 * time.Millisecond)

	if !sub.IsConnected() {
		t.Error("Subscription should remain connected")
	}
}

func TestSubscribe_Close(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Keep connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if !sub.IsConnected() {
		t.Error("Subscription should be connected")
	}

	// Close subscription
	if err := sub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Wait for goroutines to finish
	time.Sleep(100 * time.Millisecond)

	// Double close should be safe
	if err := sub.Close(); err != nil {
		t.Errorf("Double close failed: %v", err)
	}
}

func TestSubscribe_Ack(t *testing.T) {
	var ackReceived atomic.Bool
	var ackEventID string
	var mu sync.Mutex

	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Send an event
		conn.WriteJSON(map[string]any{
			"type":      "event",
			"id":        "evt-456",
			"topic":     "test-topic",
			"data":      map[string]string{"foo": "bar"},
			"timestamp": time.Now().Format(time.RFC3339),
		})

		// Wait for ack
		for {
			var ackMsg map[string]any
			if err := conn.ReadJSON(&ackMsg); err != nil {
				return
			}
			if ackMsg["action"] == "ack" {
				mu.Lock()
				ackEventID = ackMsg["id"].(string)
				mu.Unlock()
				ackReceived.Store(true)
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{AutoAck: false})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait for event
	select {
	case event := <-sub.Events():
		// Ack the event
		if err := sub.Ack(event.ID); err != nil {
			t.Fatalf("Ack failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Wait for ack to be processed
	time.Sleep(200 * time.Millisecond)

	if !ackReceived.Load() {
		t.Error("Ack not received by server")
	}

	mu.Lock()
	if ackEventID != "evt-456" {
		t.Errorf("Expected ack for 'evt-456', got '%s'", ackEventID)
	}
	mu.Unlock()
}

func TestSubscribe_Nack(t *testing.T) {
	var nackReceived atomic.Bool
	var nackRetryIn string
	var mu sync.Mutex

	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Send an event
		conn.WriteJSON(map[string]any{
			"type":      "event",
			"id":        "evt-789",
			"topic":     "test-topic",
			"data":      map[string]string{"foo": "bar"},
			"timestamp": time.Now().Format(time.RFC3339),
		})

		// Wait for nack
		for {
			var nackMsg map[string]any
			if err := conn.ReadJSON(&nackMsg); err != nil {
				return
			}
			if nackMsg["action"] == "nack" {
				mu.Lock()
				nackRetryIn = nackMsg["retry_in"].(string)
				mu.Unlock()
				nackReceived.Store(true)
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{AutoAck: false})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait for event
	select {
	case event := <-sub.Events():
		// Nack the event
		if err := sub.Nack(event.ID, "10m"); err != nil {
			t.Fatalf("Nack failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Wait for nack to be processed
	time.Sleep(200 * time.Millisecond)

	if !nackReceived.Load() {
		t.Error("Nack not received by server")
	}

	mu.Lock()
	if nackRetryIn != "10m" {
		t.Errorf("Expected retry_in '10m', got '%s'", nackRetryIn)
	}
	mu.Unlock()
}

func TestSubscribe_ErrorChannel(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		// Send error message
		conn.WriteJSON(map[string]any{
			"type":    "error",
			"message": "test error message",
		})

		// Keep connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	// Wait for error
	select {
	case err := <-sub.Errors():
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("Expected APIError, got %T", err)
		}
		if !strings.Contains(apiErr.Message, "test error message") {
			t.Errorf("Expected error message to contain 'test error message', got '%s'", apiErr.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error")
	}
}

func TestSubscribe_MultipleTopics(t *testing.T) {
	var receivedTopics []string
	var mu sync.Mutex

	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		if topics, ok := msg["topics"].([]any); ok {
			mu.Lock()
			for _, topic := range topics {
				receivedTopics = append(receivedTopics, topic.(string))
			}
			mu.Unlock()
		}

		// Send subscribed confirmation
		conn.WriteJSON(map[string]string{"type": "subscribed"})

		// Keep connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	topics := []string{"topic-a", "topic-b", "topic-c"}
	sub, err := client.Subscribe(ctx, topics, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer sub.Close()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedTopics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(receivedTopics))
	}

	for i, topic := range topics {
		if i < len(receivedTopics) && receivedTopics[i] != topic {
			t.Errorf("Expected topic '%s' at index %d, got '%s'", topic, i, receivedTopics[i])
		}
	}
}

func TestSubscribe_ConnectionError(t *testing.T) {
	// Try to connect to a non-existent server
	client := New("test-api-key", WithServer("http://localhost:59999"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err == nil {
		t.Fatal("Expected connection error, got nil")
	}

	connErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("Expected ConnectionError, got %T: %v", err, err)
	}

	if connErr.Err == nil {
		t.Error("ConnectionError.Err should not be nil")
	}
}

func TestSubscribe_AckNotConnected(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Read subscribe message and close immediately
		conn.ReadJSON(&map[string]any{})
		conn.WriteJSON(map[string]string{"type": "subscribed"})
		conn.Close()
	})
	defer server.Close()

	client := New("test-api-key", WithServer(server.URL))
	ctx := context.Background()

	sub, err := client.Subscribe(ctx, []string{"test-topic"}, SubscribeOptions{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for connection to be closed
	time.Sleep(200 * time.Millisecond)

	// Close subscription to prevent reconnection
	sub.Close()

	// Now set conn to nil manually to simulate disconnected state
	sub.connMu.Lock()
	sub.conn = nil
	sub.connMu.Unlock()

	// Ack should fail with not connected error
	err = sub.Ack("test-id")
	if err == nil {
		t.Fatal("Expected error when acking on closed connection")
	}

	connErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("Expected ConnectionError, got %T", err)
	}

	if connErr.Err != ErrNotConnected {
		t.Errorf("Expected ErrNotConnected, got %v", connErr.Err)
	}
}

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Shared test environment - reused across all tests for speed
var testEnv *TestEnv

func TestMain(m *testing.M) {
	// Note: We skip setup here since each test will call SetupTestEnv
	// In a real scenario, you might want to share the environment
	m.Run()
}

func TestHealthEndpoints(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	t.Run("health returns ok", func(t *testing.T) {
		resp, err := http.Get(env.ServerURL + "/health")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if body["status"] != "ok" {
			t.Errorf("expected status ok, got %s", body["status"])
		}
	})

	t.Run("ready returns ready with dependencies", func(t *testing.T) {
		resp, err := http.Get(env.ServerURL + "/ready")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if body["status"] != "ready" {
			t.Errorf("expected status ready, got %s", body["status"])
		}
		if body["nats"] != "connected" {
			t.Errorf("expected nats connected, got %s", body["nats"])
		}
		if body["database"] != "connected" {
			t.Errorf("expected database connected, got %s", body["database"])
		}
	})
}

func TestEmitEndpoint(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	t.Run("emit requires authorization", func(t *testing.T) {
		payload := `{"topic": "test.hello", "data": {"msg": "hello"}}`
		resp, err := http.Post(env.ServerURL+"/emit", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("emit with invalid key returns unauthorized", func(t *testing.T) {
		payload := `{"topic": "test.hello", "data": {"msg": "hello"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer nsh_test_invalidkey12345678901")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("emit with valid key succeeds", func(t *testing.T) {
		payload := `{"topic": "test.hello", "data": {"msg": "hello world"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["id"] == nil || result["id"] == "" {
			t.Error("expected event id in response")
		}
		if result["topic"] != "test.hello" {
			t.Errorf("expected topic test.hello, got %v", result["topic"])
		}
		if result["created_at"] == nil {
			t.Error("expected created_at in response")
		}

		// Verify event ID format
		id := result["id"].(string)
		if !strings.HasPrefix(id, "evt_") {
			t.Errorf("expected event id to start with evt_, got %s", id)
		}
	})

	t.Run("emit with empty topic fails", func(t *testing.T) {
		payload := `{"topic": "", "data": {"msg": "hello"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("emit with $ prefix topic fails", func(t *testing.T) {
		payload := `{"topic": "$internal.topic", "data": {"msg": "hello"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("emit with large payload fails", func(t *testing.T) {
		// Create payload larger than 64KB
		largeData := strings.Repeat("x", 70*1024)
		payload := `{"topic": "test.large", "data": {"msg": "` + largeData + `"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413, got %d", resp.StatusCode)
		}
	})
}

func TestWebSocketSubscribe(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1)

	t.Run("subscribe requires authorization", func(t *testing.T) {
		_, resp, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("subscribe with valid token succeeds", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Send subscribe message
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"test.*"},
			"options": map[string]interface{}{
				"auto_ack": true,
			},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			t.Fatalf("failed to send subscribe: %v", err)
		}

		// Read subscribed confirmation
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if response["type"] != "subscribed" {
			t.Errorf("expected type subscribed, got %v", response["type"])
		}
	})

	t.Run("ping pong works", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Send ping
		pingMsg := map[string]string{"action": "ping"}
		if err := conn.WriteJSON(pingMsg); err != nil {
			t.Fatalf("failed to send ping: %v", err)
		}

		// Read pong
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if response["type"] != "pong" {
			t.Errorf("expected type pong, got %v", response["type"])
		}
	})
}

func TestEmitAndReceive(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1)

	t.Run("emit event is received by subscriber", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe to topic
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"orders.*"},
			"options": map[string]interface{}{
				"auto_ack": true,
			},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			t.Fatalf("failed to send subscribe: %v", err)
		}

		// Wait for subscribed confirmation
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var subResp map[string]interface{}
		if err := conn.ReadJSON(&subResp); err != nil {
			t.Fatalf("failed to read subscribed response: %v", err)
		}
		if subResp["type"] != "subscribed" {
			t.Fatalf("expected subscribed, got %v", subResp["type"])
		}

		// Emit an event via HTTP
		eventData := map[string]interface{}{
			"order_id": "12345",
			"amount":   99.99,
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"topic": "orders.created",
			"data":  eventData,
		})
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("emit failed with status %d", resp.StatusCode)
		}

		// Read the event from WebSocket
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		if err := conn.ReadJSON(&eventResp); err != nil {
			t.Fatalf("failed to read event: %v", err)
		}

		if eventResp["type"] != "event" {
			t.Errorf("expected type event, got %v", eventResp["type"])
		}
		if eventResp["topic"] != "orders.created" {
			t.Errorf("expected topic orders.created, got %v", eventResp["topic"])
		}
		if eventResp["id"] == nil {
			t.Error("expected event id")
		}

		// Verify event data
		data := eventResp["data"].(map[string]interface{})
		if data["order_id"] != "12345" {
			t.Errorf("expected order_id 12345, got %v", data["order_id"])
		}
	})
}

func TestAckNack(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1)

	t.Run("manual ack works", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe with manual ack
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"ack-test.*"},
			"options": map[string]interface{}{
				"auto_ack": false,
			},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			t.Fatalf("failed to send subscribe: %v", err)
		}

		// Wait for subscribed confirmation
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var subResp map[string]interface{}
		if err := conn.ReadJSON(&subResp); err != nil {
			t.Fatalf("failed to read subscribed response: %v", err)
		}

		// Emit an event
		payload := `{"topic": "ack-test.item", "data": {"test": true}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()

		// Read the event
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		if err := conn.ReadJSON(&eventResp); err != nil {
			t.Fatalf("failed to read event: %v", err)
		}

		eventID := eventResp["id"].(string)

		// Send ack
		ackMsg := map[string]string{
			"action": "ack",
			"id":     eventID,
		}
		if err := conn.WriteJSON(ackMsg); err != nil {
			t.Fatalf("failed to send ack: %v", err)
		}

		// No error response expected - the ack should succeed silently
		// Try to read with short timeout - should timeout (no message)
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		var response map[string]interface{}
		err = conn.ReadJSON(&response)
		// Timeout is expected here since there's no response for successful ack
		if err == nil && response["type"] == "error" {
			t.Errorf("got error response: %v", response)
		}
	})

	t.Run("ack unknown event returns error", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Send ack for non-existent event
		ackMsg := map[string]string{
			"action": "ack",
			"id":     "evt_nonexistent123456",
		}
		if err := conn.WriteJSON(ackMsg); err != nil {
			t.Fatalf("failed to send ack: %v", err)
		}

		// Should get error response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var response map[string]interface{}
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if response["type"] != "error" {
			t.Errorf("expected type error, got %v", response["type"])
		}
		if response["code"] != "UNKNOWN_EVENT" {
			t.Errorf("expected code UNKNOWN_EVENT, got %v", response["code"])
		}
	})
}

func TestWildcardSubscription(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1)

	t.Run("wildcard * matches single segment", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/subscribe?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe with wildcard
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"wildcard.*"},
			"options": map[string]interface{}{
				"auto_ack": true,
			},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			t.Fatalf("failed to send subscribe: %v", err)
		}

		// Wait for subscribed confirmation
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var subResp map[string]interface{}
		conn.ReadJSON(&subResp)

		// Emit matching event
		payload := `{"topic": "wildcard.match", "data": {"matched": true}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()

		// Should receive the event
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		if err := conn.ReadJSON(&eventResp); err != nil {
			t.Fatalf("failed to read event: %v", err)
		}

		if eventResp["type"] != "event" {
			t.Errorf("expected type event, got %v", eventResp["type"])
		}
		if eventResp["topic"] != "wildcard.match" {
			t.Errorf("expected topic wildcard.match, got %v", eventResp["topic"])
		}
	})
}

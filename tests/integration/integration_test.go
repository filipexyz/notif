//go:build integration

// Package integration provides integration tests that run against
// local docker-compose services (make dev).
//
// Run with: go test -tags=integration ./tests/integration/...
// Requires: docker-compose services running (make dev && make seed && make run)
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// TestAPIKey is the seeded test key from scripts/seed.sql
	TestAPIKey = "nsh_test_abcdefghij12345678901234"
)

var baseURL string

func TestMain(m *testing.M) {
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Wait for server to be ready
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				os.Exit(m.Run())
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	println("Server not ready at", baseURL)
	os.Exit(1)
}

func TestHealth(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestReady(t *testing.T) {
	resp, err := http.Get(baseURL + "/ready")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "ready" {
		t.Errorf("expected ready, got %s", body["status"])
	}
	if body["nats"] != "connected" {
		t.Errorf("expected nats connected, got %s", body["nats"])
	}
	if body["database"] != "connected" {
		t.Errorf("expected database connected, got %s", body["database"])
	}
}

func TestEmit_Unauthorized(t *testing.T) {
	payload := `{"topic": "test.hello", "data": {"msg": "hello"}}`
	resp, err := http.Post(baseURL+"/api/v1/emit", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEmit_InvalidKey(t *testing.T) {
	payload := `{"topic": "test.hello", "data": {"msg": "hello"}}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer nsh_test_invalidkey12345678901")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestEmit_Success(t *testing.T) {
	payload := `{"topic": "test.integration", "data": {"msg": "hello world"}}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["id"] == nil || result["id"] == "" {
		t.Error("expected event id")
	}

	id := result["id"].(string)
	if !strings.HasPrefix(id, "evt_") {
		t.Errorf("expected id to start with evt_, got %s", id)
	}

	if result["topic"] != "test.integration" {
		t.Errorf("expected topic test.integration, got %v", result["topic"])
	}
}

func TestEmit_EmptyTopic(t *testing.T) {
	payload := `{"topic": "", "data": {"msg": "hello"}}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEmit_InvalidTopic(t *testing.T) {
	payload := `{"topic": "$internal", "data": {"msg": "hello"}}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebSocket_Unauthorized(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"/ws", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebSocket_Subscribe(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Subscribe
	msg := map[string]interface{}{
		"action": "subscribe",
		"topics": []string{"integration.*"},
		"options": map[string]interface{}{
			"auto_ack": true,
		},
	}
	conn.WriteJSON(msg)

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if resp["type"] != "subscribed" {
		t.Errorf("expected subscribed, got %v", resp["type"])
	}
}

func TestWebSocket_PingPong(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	conn.WriteJSON(map[string]string{"action": "ping"})

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if resp["type"] != "pong" {
		t.Errorf("expected pong, got %v", resp["type"])
	}
}

func TestEmitAndReceive(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)

	// Connect WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Subscribe
	conn.WriteJSON(map[string]interface{}{
		"action": "subscribe",
		"topics": []string{"e2e.*"},
		"options": map[string]interface{}{
			"auto_ack": true,
		},
	})

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var subResp map[string]interface{}
	conn.ReadJSON(&subResp)
	if subResp["type"] != "subscribed" {
		t.Fatalf("expected subscribed, got %v", subResp["type"])
	}

	// Emit via HTTP
	eventData := map[string]interface{}{
		"test_id": "e2e-123",
		"value":   42,
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"topic": "e2e.test",
		"data":  eventData,
	})
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("emit failed: %v", err)
	}
	resp.Body.Close()

	// Read event from WebSocket
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eventResp map[string]interface{}
	if err := conn.ReadJSON(&eventResp); err != nil {
		t.Fatalf("read event failed: %v", err)
	}

	if eventResp["type"] != "event" {
		t.Errorf("expected event, got %v", eventResp["type"])
	}
	if eventResp["topic"] != "e2e.test" {
		t.Errorf("expected e2e.test, got %v", eventResp["topic"])
	}

	data := eventResp["data"].(map[string]interface{})
	if data["test_id"] != "e2e-123" {
		t.Errorf("expected test_id e2e-123, got %v", data["test_id"])
	}
}

func TestManualAck(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)

	// Connect with manual ack
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	conn.WriteJSON(map[string]interface{}{
		"action": "subscribe",
		"topics": []string{"ack-test.*"},
		"options": map[string]interface{}{
			"auto_ack": false,
		},
	})

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var subResp map[string]interface{}
	conn.ReadJSON(&subResp)

	// Emit event
	payload := `{"topic": "ack-test.manual", "data": {"ack": true}}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/emit", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	// Read event
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eventResp map[string]interface{}
	if err := conn.ReadJSON(&eventResp); err != nil {
		t.Fatalf("read event failed: %v", err)
	}

	eventID := eventResp["id"].(string)

	// Ack the event
	conn.WriteJSON(map[string]string{
		"action": "ack",
		"id":     eventID,
	})

	// Should not get error (successful ack is silent)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var ackResp map[string]interface{}
	err = conn.ReadJSON(&ackResp)
	// Timeout expected for successful ack
	if err == nil && ackResp["type"] == "error" {
		t.Errorf("unexpected error: %v", ackResp)
	}
}

func TestAckUnknownEvent(t *testing.T) {
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Ack non-existent event
	conn.WriteJSON(map[string]string{
		"action": "ack",
		"id":     "evt_nonexistent123456",
	})

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if resp["type"] != "error" {
		t.Errorf("expected error, got %v", resp["type"])
	}
	if resp["code"] != "UNKNOWN_EVENT" {
		t.Errorf("expected UNKNOWN_EVENT, got %v", resp["code"])
	}
}

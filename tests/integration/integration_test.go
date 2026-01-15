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
	"fmt"
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
	// Format: nsh_ + 28 alphanumeric chars = 32 total
	TestAPIKey = "nsh_testkey1234567890abcdefghijk"
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

// ============================================================================
// Scheduled Events Tests
// ============================================================================

func TestSchedule_Create_WithIn(t *testing.T) {
	payload := `{"topic": "test.scheduled", "data": {"msg": "scheduled"}, "in": "30s"}`
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["id"] == nil || result["id"] == "" {
		t.Error("expected schedule id")
	}

	id := result["id"].(string)
	if !strings.HasPrefix(id, "sch_") {
		t.Errorf("expected id to start with sch_, got %s", id)
	}

	if result["topic"] != "test.scheduled" {
		t.Errorf("expected topic test.scheduled, got %v", result["topic"])
	}

	if result["scheduled_for"] == nil {
		t.Error("expected scheduled_for in response")
	}
}

func TestSchedule_Create_WithAt(t *testing.T) {
	scheduledFor := time.Now().Add(1 * time.Minute).UTC().Format(time.RFC3339)
	payload := fmt.Sprintf(`{"topic": "test.scheduled.at", "data": {"msg": "scheduled at"}, "scheduled_for": "%s"}`, scheduledFor)
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["id"] == nil {
		t.Error("expected schedule id")
	}

	if result["scheduled_for"] == nil {
		t.Error("expected scheduled_for in response")
	}
}

func TestSchedule_Create_Unauthorized(t *testing.T) {
	payload := `{"topic": "test.scheduled", "data": {"msg": "scheduled"}, "in": "30s"}`
	resp, err := http.Post(baseURL+"/api/v1/schedules", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSchedule_List(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules", nil)
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

	if result["schedules"] == nil {
		t.Error("expected schedules array")
	}

	if result["count"] == nil {
		t.Error("expected count field")
	}
}

func TestSchedule_ListByStatus(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules?status=pending", nil)
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

	// All returned schedules should have status=pending
	schedules := result["schedules"].([]interface{})
	for _, s := range schedules {
		schedule := s.(map[string]interface{})
		if schedule["status"] != "pending" {
			t.Errorf("expected all schedules to have status pending, got %v", schedule["status"])
		}
	}
}

func TestSchedule_Get(t *testing.T) {
	// First create a schedule
	payload := `{"topic": "test.get", "data": {"msg": "get test"}, "in": "5m"}`
	createReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&created)
	scheduleID := created["id"].(string)

	// Now get it
	getReq, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules/"+scheduleID, nil)
	getReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("expected 200, got %d: %s", getResp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&result)

	if result["id"] != scheduleID {
		t.Errorf("expected id %s, got %v", scheduleID, result["id"])
	}

	if result["topic"] != "test.get" {
		t.Errorf("expected topic test.get, got %v", result["topic"])
	}
}

func TestSchedule_GetNotFound(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules/sch_nonexistent123456789", nil)
	req.Header.Set("Authorization", "Bearer "+TestAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSchedule_Cancel(t *testing.T) {
	// First create a schedule
	payload := `{"topic": "test.cancel", "data": {"msg": "cancel test"}, "in": "5m"}`
	createReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&created)
	scheduleID := created["id"].(string)

	// Cancel it
	cancelReq, _ := http.NewRequest("DELETE", baseURL+"/api/v1/schedules/"+scheduleID, nil)
	cancelReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	cancelResp, err := http.DefaultClient.Do(cancelReq)
	if err != nil {
		t.Fatalf("cancel request failed: %v", err)
	}
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cancelResp.Body)
		t.Fatalf("expected 200, got %d: %s", cancelResp.StatusCode, body)
	}

	// Verify it's cancelled
	getReq, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules/"+scheduleID, nil)
	getReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	getResp, _ := http.DefaultClient.Do(getReq)
	defer getResp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&result)

	if result["status"] != "cancelled" {
		t.Errorf("expected status cancelled, got %v", result["status"])
	}
}

func TestSchedule_Run(t *testing.T) {
	// First create a schedule
	payload := `{"topic": "test.run", "data": {"msg": "run test"}, "in": "10m"}`
	createReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&created)
	scheduleID := created["id"].(string)

	// Run it immediately
	runReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules/"+scheduleID+"/run", nil)
	runReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	runResp, err := http.DefaultClient.Do(runReq)
	if err != nil {
		t.Fatalf("run request failed: %v", err)
	}
	defer runResp.Body.Close()

	if runResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runResp.Body)
		t.Fatalf("expected 200, got %d: %s", runResp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(runResp.Body).Decode(&result)

	if result["schedule_id"] != scheduleID {
		t.Errorf("expected schedule_id %s, got %v", scheduleID, result["schedule_id"])
	}

	eventID := result["event_id"].(string)
	if !strings.HasPrefix(eventID, "evt_") {
		t.Errorf("expected event_id to start with evt_, got %s", eventID)
	}

	// Verify schedule is completed
	getReq, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules/"+scheduleID, nil)
	getReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	getResp, _ := http.DefaultClient.Do(getReq)
	defer getResp.Body.Close()

	var schedule map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&schedule)

	if schedule["status"] != "completed" {
		t.Errorf("expected status completed, got %v", schedule["status"])
	}
}

func TestSchedule_RunAlreadyCompleted(t *testing.T) {
	// First create and run a schedule
	payload := `{"topic": "test.run.twice", "data": {"msg": "run twice test"}, "in": "10m"}`
	createReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	createResp, _ := http.DefaultClient.Do(createReq)
	defer createResp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&created)
	scheduleID := created["id"].(string)

	// Run it first time
	runReq1, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules/"+scheduleID+"/run", nil)
	runReq1.Header.Set("Authorization", "Bearer "+TestAPIKey)
	runResp1, _ := http.DefaultClient.Do(runReq1)
	runResp1.Body.Close()

	// Try to run it again
	runReq2, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules/"+scheduleID+"/run", nil)
	runReq2.Header.Set("Authorization", "Bearer "+TestAPIKey)

	runResp2, err := http.DefaultClient.Do(runReq2)
	if err != nil {
		t.Fatalf("second run request failed: %v", err)
	}
	defer runResp2.Body.Close()

	// Should fail because it's already completed
	if runResp2.StatusCode != http.StatusNotFound && runResp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 404 or 400 for already completed schedule, got %d", runResp2.StatusCode)
	}
}

func TestSchedule_Stats(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/stats/schedules", nil)
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

	// Should have counts for each status
	expectedFields := []string{"pending", "completed", "cancelled", "failed"}
	for _, field := range expectedFields {
		if result[field] == nil {
			t.Errorf("expected field %s in stats", field)
		}
	}
}

func TestSchedule_AutoExecution(t *testing.T) {
	// This test verifies that the scheduler worker executes pending events
	// Create a schedule for 5 seconds from now
	payload := `{"topic": "test.auto", "data": {"msg": "auto execution"}, "in": "5s"}`
	createReq, _ := http.NewRequest("POST", baseURL+"/api/v1/schedules", strings.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	defer createResp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&created)
	scheduleID := created["id"].(string)

	// Wait for scheduler to pick it up (scheduler runs every 10s)
	// We wait up to 20 seconds
	deadline := time.Now().Add(20 * time.Second)
	var status string

	for time.Now().Before(deadline) {
		getReq, _ := http.NewRequest("GET", baseURL+"/api/v1/schedules/"+scheduleID, nil)
		getReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

		getResp, err := http.DefaultClient.Do(getReq)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var schedule map[string]interface{}
		json.NewDecoder(getResp.Body).Decode(&schedule)
		getResp.Body.Close()

		status = schedule["status"].(string)
		if status == "completed" {
			return // Success!
		}

		time.Sleep(1 * time.Second)
	}

	t.Errorf("expected status completed within 20s, got %s", status)
}

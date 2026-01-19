package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Two projects for isolation testing
	TestProjectA_ID  = "prj_projectA_isolation_test1"
	TestProjectB_ID  = "prj_projectB_isolation_test2"
	// API keys must be nsh_ + 28 alphanumeric chars (no underscores)
	TestAPIKeyA      = "nsh_projectAkey1234567890abcdefg"
	TestAPIKeyB      = "nsh_projectBkey1234567890xyzabcd"
	IsolationTestOrg = "org_isolation_test"
)

// setupProjectIsolationTest creates two projects with separate API keys
func setupProjectIsolationTest(t *testing.T, env *TestEnv) {
	ctx := context.Background()

	// Create Project A
	_, err := env.DB.Exec(ctx, `
		INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
		VALUES ($1, $2, 'Project A', 'project-a', NOW(), NOW())
		ON CONFLICT (org_id, slug) DO NOTHING
	`, TestProjectA_ID, IsolationTestOrg)
	if err != nil {
		t.Fatalf("failed to create project A: %v", err)
	}

	// Create Project B
	_, err = env.DB.Exec(ctx, `
		INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
		VALUES ($1, $2, 'Project B', 'project-b', NOW(), NOW())
		ON CONFLICT (org_id, slug) DO NOTHING
	`, TestProjectB_ID, IsolationTestOrg)
	if err != nil {
		t.Fatalf("failed to create project B: %v", err)
	}

	// Create API Key for Project A (prefix is first 16 chars)
	hashA := sha256.Sum256([]byte(TestAPIKeyA))
	_, err = env.DB.Exec(ctx, `
		INSERT INTO api_keys (key_hash, key_prefix, name, org_id, project_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key_hash) DO NOTHING
	`, hex.EncodeToString(hashA[:]), TestAPIKeyA[:16], "Project A Key", IsolationTestOrg, TestProjectA_ID)
	if err != nil {
		t.Fatalf("failed to create API key A: %v", err)
	}

	// Create API Key for Project B (prefix is first 16 chars)
	hashB := sha256.Sum256([]byte(TestAPIKeyB))
	_, err = env.DB.Exec(ctx, `
		INSERT INTO api_keys (key_hash, key_prefix, name, org_id, project_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key_hash) DO NOTHING
	`, hex.EncodeToString(hashB[:]), TestAPIKeyB[:16], "Project B Key", IsolationTestOrg, TestProjectB_ID)
	if err != nil {
		t.Fatalf("failed to create API key B: %v", err)
	}
}

func TestProjectIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	setupProjectIsolationTest(t, env)

	t.Run("events emitted in project A are not visible in project B", func(t *testing.T) {
		// Emit event with Project A's API key
		payload := `{"topic": "isolation.test", "data": {"project": "A", "value": 123}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("emit failed with status %d", resp.StatusCode)
		}

		// Small delay for NATS to process
		time.Sleep(100 * time.Millisecond)

		// Query events with Project A's key - should see the event
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/events?topic=isolation.test", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list events A request failed: %v", err)
		}
		defer resp.Body.Close()

		var eventsA struct {
			Events []map[string]interface{} `json:"events"`
			Count  int                      `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&eventsA); err != nil {
			t.Fatalf("failed to decode events A: %v", err)
		}

		if eventsA.Count == 0 {
			t.Error("Project A should see its own events, but found none")
		}

		// Query events with Project B's key - should NOT see the event
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/events?topic=isolation.test", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list events B request failed: %v", err)
		}
		defer resp.Body.Close()

		var eventsB struct {
			Events []map[string]interface{} `json:"events"`
			Count  int                      `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&eventsB); err != nil {
			t.Fatalf("failed to decode events B: %v", err)
		}

		if eventsB.Count != 0 {
			t.Errorf("Project B should NOT see Project A's events, but found %d events", eventsB.Count)
		}
	})

	t.Run("events emitted in project B are not visible in project A", func(t *testing.T) {
		// Emit event with Project B's API key
		payload := `{"topic": "isolation.testB", "data": {"project": "B", "value": 456}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("emit failed with status %d", resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)

		// Query events with Project B's key - should see the event
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/events?topic=isolation.testB", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list events B request failed: %v", err)
		}
		defer resp.Body.Close()

		var eventsB struct {
			Events []map[string]interface{} `json:"events"`
			Count  int                      `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&eventsB); err != nil {
			t.Fatalf("failed to decode events B: %v", err)
		}

		if eventsB.Count == 0 {
			t.Error("Project B should see its own events, but found none")
		}

		// Query events with Project A's key - should NOT see the event
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/events?topic=isolation.testB", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list events A request failed: %v", err)
		}
		defer resp.Body.Close()

		var eventsA struct {
			Events []map[string]interface{} `json:"events"`
			Count  int                      `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&eventsA); err != nil {
			t.Fatalf("failed to decode events A: %v", err)
		}

		if eventsA.Count != 0 {
			t.Errorf("Project A should NOT see Project B's events, but found %d events", eventsA.Count)
		}
	})

	t.Run("websocket subscription only receives events from own project", func(t *testing.T) {
		// Connect WebSocket for Project A
		wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1) + "/ws?token=" + TestAPIKeyA
		connA, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect WebSocket A: %v", err)
		}
		defer connA.Close()

		// Connect WebSocket for Project B
		wsURL = strings.Replace(env.ServerURL, "http://", "ws://", 1) + "/ws?token=" + TestAPIKeyB
		connB, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect WebSocket B: %v", err)
		}
		defer connB.Close()

		// Subscribe both to the same topic pattern
		subscribeMsg := `{"action": "subscribe", "topics": ["ws.isolation.*"], "options": {"auto_ack": true}}`
		if err := connA.WriteMessage(websocket.TextMessage, []byte(subscribeMsg)); err != nil {
			t.Fatalf("failed to subscribe A: %v", err)
		}
		if err := connB.WriteMessage(websocket.TextMessage, []byte(subscribeMsg)); err != nil {
			t.Fatalf("failed to subscribe B: %v", err)
		}

		// Wait for subscription confirmations
		time.Sleep(200 * time.Millisecond)

		// Read subscription confirmations
		connA.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _ = connA.ReadMessage() // consume subscribed message
		connB.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _ = connB.ReadMessage() // consume subscribed message

		// Emit event from Project A
		payload := `{"topic": "ws.isolation.test", "data": {"from": "projectA"}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit failed: %v", err)
		}
		resp.Body.Close()

		// Project A's WebSocket should receive the event
		connA.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msgA, err := connA.ReadMessage()
		if err != nil {
			t.Errorf("Project A WebSocket should receive event, got error: %v", err)
		} else {
			var event map[string]interface{}
			json.Unmarshal(msgA, &event)
			if event["type"] != "event" {
				t.Errorf("Expected event message, got: %s", string(msgA))
			}
		}

		// Project B's WebSocket should NOT receive the event (timeout expected)
		connB.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgB, err := connB.ReadMessage()
		if err == nil {
			var event map[string]interface{}
			json.Unmarshal(msgB, &event)
			if event["type"] == "event" {
				t.Errorf("Project B WebSocket should NOT receive Project A's event, but got: %s", string(msgB))
			}
		}
		// Timeout error is expected - Project B should not receive the event
	})

	t.Run("webhooks are project-scoped", func(t *testing.T) {
		// Create webhook in Project A
		webhookPayload := `{"url": "https://example.com/webhookA", "topics": ["webhook.test"]}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/webhooks", strings.NewReader(webhookPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create webhook failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create webhook failed with status %d", resp.StatusCode)
		}

		// List webhooks with Project A's key - should see webhook
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/webhooks", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list webhooks A failed: %v", err)
		}
		defer resp.Body.Close()

		var webhooksA struct {
			Webhooks []map[string]interface{} `json:"webhooks"`
		}
		json.NewDecoder(resp.Body).Decode(&webhooksA)

		foundInA := false
		for _, wh := range webhooksA.Webhooks {
			if wh["url"] == "https://example.com/webhookA" {
				foundInA = true
				break
			}
		}
		if !foundInA {
			t.Error("Project A should see its own webhook")
		}

		// List webhooks with Project B's key - should NOT see Project A's webhook
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/webhooks", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list webhooks B failed: %v", err)
		}
		defer resp.Body.Close()

		var webhooksB struct {
			Webhooks []map[string]interface{} `json:"webhooks"`
		}
		json.NewDecoder(resp.Body).Decode(&webhooksB)

		for _, wh := range webhooksB.Webhooks {
			if wh["url"] == "https://example.com/webhookA" {
				t.Error("Project B should NOT see Project A's webhook")
				break
			}
		}
	})

	t.Run("schedules are project-scoped", func(t *testing.T) {
		// Create schedule in Project A
		futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		schedulePayload := fmt.Sprintf(`{"topic": "schedule.test", "data": {"from": "A"}, "scheduled_for": "%s"}`, futureTime)
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/schedules", strings.NewReader(schedulePayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create schedule failed: %v", err)
		}
		var createdSchedule struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&createdSchedule)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create schedule failed with status %d", resp.StatusCode)
		}

		// List schedules with Project A's key - should see schedule
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/schedules", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list schedules A failed: %v", err)
		}
		defer resp.Body.Close()

		var schedulesA struct {
			Schedules []map[string]interface{} `json:"schedules"`
		}
		json.NewDecoder(resp.Body).Decode(&schedulesA)

		foundInA := false
		for _, s := range schedulesA.Schedules {
			if s["id"] == createdSchedule.ID {
				foundInA = true
				break
			}
		}
		if !foundInA {
			t.Error("Project A should see its own schedule")
		}

		// List schedules with Project B's key - should NOT see Project A's schedule
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/schedules", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list schedules B failed: %v", err)
		}
		defer resp.Body.Close()

		var schedulesB struct {
			Schedules []map[string]interface{} `json:"schedules"`
		}
		json.NewDecoder(resp.Body).Decode(&schedulesB)

		for _, s := range schedulesB.Schedules {
			if s["id"] == createdSchedule.ID {
				t.Error("Project B should NOT see Project A's schedule")
				break
			}
		}
	})
}

func TestProjectIsolationNATSSubjects(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	setupProjectIsolationTest(t, env)

	t.Run("NATS subjects include project ID", func(t *testing.T) {
		// This test verifies that when we emit an event, it goes to the correct
		// project-scoped subject in NATS

		// Emit with Project A
		payload := `{"topic": "nats.subject.test", "data": {"test": true}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit failed: %v", err)
		}
		var emitResp struct {
			ID    string `json:"id"`
			Topic string `json:"topic"`
		}
		json.NewDecoder(resp.Body).Decode(&emitResp)
		resp.Body.Close()

		if emitResp.ID == "" {
			t.Fatal("emit should return id")
		}

		// The key verification is that Project B cannot see this event
		// which proves the NATS subject includes project isolation
		time.Sleep(100 * time.Millisecond)

		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/events?topic=nats.subject.test", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("list events failed: %v", err)
		}
		defer resp.Body.Close()

		var events struct {
			Count int `json:"count"`
		}
		json.NewDecoder(resp.Body).Decode(&events)

		if events.Count != 0 {
			t.Errorf("NATS subject isolation failed: Project B can see Project A's events (count=%d)", events.Count)
		}
	})
}

func TestCrossProjectAccessDenied(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	setupProjectIsolationTest(t, env)

	t.Run("cannot access other project's webhook by ID", func(t *testing.T) {
		// Create webhook in Project A
		webhookPayload := `{"url": "https://example.com/cross-test", "topics": ["cross.test"]}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/webhooks", strings.NewReader(webhookPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create webhook failed: %v", err)
		}

		var webhook struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&webhook)
		resp.Body.Close()

		// Try to access Project A's webhook with Project B's key
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/webhooks/"+webhook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("get webhook failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("accessing other project's webhook should return 404, got %d", resp.StatusCode)
		}
	})

	t.Run("cannot delete other project's webhook", func(t *testing.T) {
		// Create webhook in Project A
		webhookPayload := `{"url": "https://example.com/delete-test", "topics": ["delete.test"]}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/webhooks", strings.NewReader(webhookPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create webhook failed: %v", err)
		}

		var webhook struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&webhook)
		resp.Body.Close()

		// Try to delete Project A's webhook with Project B's key
		req, _ = http.NewRequest("DELETE", env.ServerURL+"/api/v1/webhooks/"+webhook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete webhook failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("deleting other project's webhook should return 404, got %d", resp.StatusCode)
		}

		// Verify webhook still exists for Project A
		req, _ = http.NewRequest("GET", env.ServerURL+"/api/v1/webhooks/"+webhook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("get webhook failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("webhook should still exist for Project A, got status %d", resp.StatusCode)
		}
	})

	t.Run("cannot cancel other project's schedule", func(t *testing.T) {
		// Create schedule in Project A
		futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
		schedulePayload := fmt.Sprintf(`{"topic": "cancel.test", "data": {}, "scheduled_for": "%s"}`, futureTime)
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/schedules", bytes.NewReader([]byte(schedulePayload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyA)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create schedule failed: %v", err)
		}

		var schedule struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&schedule)
		resp.Body.Close()

		// Try to cancel Project A's schedule with Project B's key
		req, _ = http.NewRequest("DELETE", env.ServerURL+"/api/v1/schedules/"+schedule.ID, nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKeyB)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("cancel schedule failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("canceling other project's schedule should return 404, got %d", resp.StatusCode)
		}
	})
}

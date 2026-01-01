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

func TestEventDeliveryTracking(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	wsURL := strings.Replace(env.ServerURL, "http://", "ws://", 1)

	t.Run("websocket delivery is tracked in database", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe with auto-ack
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"delivery-track.*"},
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

		// Emit an event
		eventData := map[string]interface{}{
			"test_id": "track-001",
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"topic": "delivery-track.test",
			"data":  eventData,
		})
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("emit request failed: %v", err)
		}
		defer resp.Body.Close()

		var emitResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&emitResult)
		eventID := emitResult["id"].(string)

		// Read the event from WebSocket
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		if err := conn.ReadJSON(&eventResp); err != nil {
			t.Fatalf("failed to read event: %v", err)
		}

		if eventResp["type"] != "event" {
			t.Fatalf("expected type event, got %v", eventResp["type"])
		}

		// Wait for delivery to be recorded
		time.Sleep(200 * time.Millisecond)

		// Check deliveries via API
		deliveriesReq, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/"+eventID+"/deliveries", nil)
		deliveriesReq.Header.Set("Authorization", "Bearer "+TestAPIKey)

		deliveriesResp, err := http.DefaultClient.Do(deliveriesReq)
		if err != nil {
			t.Fatalf("failed to get deliveries: %v", err)
		}
		defer deliveriesResp.Body.Close()

		if deliveriesResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(deliveriesResp.Body)
			t.Fatalf("expected status 200, got %d: %s", deliveriesResp.StatusCode, body)
		}

		var deliveriesResult map[string]interface{}
		json.NewDecoder(deliveriesResp.Body).Decode(&deliveriesResult)

		deliveries := deliveriesResult["deliveries"].([]interface{})
		if len(deliveries) == 0 {
			t.Fatal("expected at least 1 delivery record")
		}

		// Verify delivery record
		delivery := deliveries[0].(map[string]interface{})
		if delivery["receiver_type"] != "websocket" {
			t.Errorf("expected receiver_type websocket, got %v", delivery["receiver_type"])
		}
		if delivery["status"] != "acked" {
			t.Errorf("expected status acked (auto-ack), got %v", delivery["status"])
		}
		if delivery["event_id"] != eventID {
			t.Errorf("expected event_id %s, got %v", eventID, delivery["event_id"])
		}
	})

	t.Run("manual ack updates delivery status", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe with manual ack
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"manual-ack-track.*"},
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
		conn.ReadJSON(&subResp)

		// Emit an event
		payload := `{"topic": "manual-ack-track.test", "data": {"manual": true}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, _ := http.DefaultClient.Do(req)
		var emitResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&emitResult)
		resp.Body.Close()
		eventID := emitResult["id"].(string)

		// Read the event
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		conn.ReadJSON(&eventResp)

		// Check delivery status before ack (should be 'delivered')
		time.Sleep(100 * time.Millisecond)
		deliveriesReq, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/"+eventID+"/deliveries", nil)
		deliveriesReq.Header.Set("Authorization", "Bearer "+TestAPIKey)
		deliveriesResp, _ := http.DefaultClient.Do(deliveriesReq)
		var deliveriesResult map[string]interface{}
		json.NewDecoder(deliveriesResp.Body).Decode(&deliveriesResult)
		deliveriesResp.Body.Close()

		deliveries := deliveriesResult["deliveries"].([]interface{})
		if len(deliveries) > 0 {
			delivery := deliveries[0].(map[string]interface{})
			if delivery["status"] != "delivered" {
				t.Logf("status before ack: %v (expected 'delivered')", delivery["status"])
			}
		}

		// Send ack
		ackMsg := map[string]string{
			"action": "ack",
			"id":     eventID,
		}
		conn.WriteJSON(ackMsg)

		// Wait for ack to be processed
		time.Sleep(200 * time.Millisecond)

		// Check delivery status after ack (should be 'acked')
		deliveriesReq2, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/"+eventID+"/deliveries", nil)
		deliveriesReq2.Header.Set("Authorization", "Bearer "+TestAPIKey)
		deliveriesResp2, _ := http.DefaultClient.Do(deliveriesReq2)
		var deliveriesResult2 map[string]interface{}
		json.NewDecoder(deliveriesResp2.Body).Decode(&deliveriesResult2)
		deliveriesResp2.Body.Close()

		deliveries2 := deliveriesResult2["deliveries"].([]interface{})
		if len(deliveries2) == 0 {
			t.Fatal("expected at least 1 delivery record after ack")
		}

		delivery2 := deliveries2[0].(map[string]interface{})
		if delivery2["status"] != "acked" {
			t.Errorf("expected status acked after ack, got %v", delivery2["status"])
		}
		if delivery2["acked_at"] == nil {
			t.Error("expected acked_at to be set")
		}
	})

	t.Run("nack moves to dlq after max retries", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe with manual ack and max_retries=1 (nack goes straight to DLQ)
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"nack-track.*"},
			"options": map[string]interface{}{
				"auto_ack":    false,
				"max_retries": 1,
			},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			t.Fatalf("failed to send subscribe: %v", err)
		}

		// Wait for subscribed confirmation
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var subResp map[string]interface{}
		conn.ReadJSON(&subResp)

		// Emit an event
		payload := `{"topic": "nack-track.test", "data": {"will_nack": true}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, _ := http.DefaultClient.Do(req)
		var emitResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&emitResult)
		resp.Body.Close()
		eventID := emitResult["id"].(string)

		// Read the event
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		conn.ReadJSON(&eventResp)

		// Send nack - with max_retries=1, this goes directly to DLQ
		nackMsg := map[string]interface{}{
			"action":   "nack",
			"id":       eventID,
			"retry_in": "100ms",
		}
		conn.WriteJSON(nackMsg)

		// Wait for nack to be processed
		time.Sleep(200 * time.Millisecond)

		// Check delivery status (should be 'dlq' since max_retries=1)
		deliveriesReq, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/"+eventID+"/deliveries", nil)
		deliveriesReq.Header.Set("Authorization", "Bearer "+TestAPIKey)
		deliveriesResp, _ := http.DefaultClient.Do(deliveriesReq)
		var deliveriesResult map[string]interface{}
		json.NewDecoder(deliveriesResp.Body).Decode(&deliveriesResult)
		deliveriesResp.Body.Close()

		deliveries := deliveriesResult["deliveries"].([]interface{})
		if len(deliveries) == 0 {
			t.Fatal("expected at least 1 delivery record after nack")
		}

		delivery := deliveries[0].(map[string]interface{})
		if delivery["status"] != "dlq" {
			t.Errorf("expected status dlq (max_retries=1 exhausted), got %v", delivery["status"])
		}
		if delivery["error"] == nil {
			t.Error("expected error field to be set")
		}
	})

	t.Run("deliveries API returns receiver_type", func(t *testing.T) {
		// Connect WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/ws?token="+TestAPIKey, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Subscribe
		subscribeMsg := map[string]interface{}{
			"action": "subscribe",
			"topics": []string{"receiver-type-test.*"},
			"options": map[string]interface{}{
				"auto_ack": true,
			},
		}
		conn.WriteJSON(subscribeMsg)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var subResp map[string]interface{}
		conn.ReadJSON(&subResp)

		// Emit an event
		payload := `{"topic": "receiver-type-test.event", "data": {}}`
		req, _ := http.NewRequest("POST", env.ServerURL+"/api/v1/emit", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, _ := http.DefaultClient.Do(req)
		var emitResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&emitResult)
		resp.Body.Close()
		eventID := emitResult["id"].(string)

		// Read the event
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var eventResp map[string]interface{}
		conn.ReadJSON(&eventResp)

		time.Sleep(200 * time.Millisecond)

		// Check deliveries
		deliveriesReq, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/"+eventID+"/deliveries", nil)
		deliveriesReq.Header.Set("Authorization", "Bearer "+TestAPIKey)
		deliveriesResp, _ := http.DefaultClient.Do(deliveriesReq)
		var deliveriesResult map[string]interface{}
		json.NewDecoder(deliveriesResp.Body).Decode(&deliveriesResult)
		deliveriesResp.Body.Close()

		deliveries := deliveriesResult["deliveries"].([]interface{})
		if len(deliveries) == 0 {
			t.Fatal("expected at least 1 delivery record")
		}

		delivery := deliveries[0].(map[string]interface{})

		// Verify all expected fields
		requiredFields := []string{"id", "event_id", "receiver_type", "status", "attempt", "created_at"}
		for _, field := range requiredFields {
			if delivery[field] == nil {
				t.Errorf("expected field %s in delivery response", field)
			}
		}

		// Websocket-specific fields
		if delivery["receiver_type"] == "websocket" {
			// Should have client_id or consumer_name
			if delivery["client_id"] == nil && delivery["consumer_name"] == nil {
				t.Error("expected client_id or consumer_name for websocket delivery")
			}
		}
	})
}

func TestEventDeliveriesEndpoint(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	t.Run("deliveries endpoint requires event id", func(t *testing.T) {
		// This should return 404 for empty path
		req, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events//deliveries", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Empty event ID should fail
		if resp.StatusCode == http.StatusOK {
			t.Error("expected non-200 status for empty event id")
		}
	})

	t.Run("deliveries for non-existent event returns empty array", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/evt_nonexistent123/deliveries", nil)
		req.Header.Set("Authorization", "Bearer "+TestAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		count := int(result["count"].(float64))
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}

		deliveries := result["deliveries"].([]interface{})
		if len(deliveries) != 0 {
			t.Errorf("expected 0 deliveries, got %d", len(deliveries))
		}
	})

	t.Run("deliveries requires authorization", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.ServerURL+"/api/v1/events/evt_test/deliveries", nil)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

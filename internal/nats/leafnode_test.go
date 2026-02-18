package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// TestLeafnodeSubjectFlow verifies that two NATS servers connected via
// leafnode correctly pass messages bidirectionally.
//
// This is the core test proving the config-only approach works:
// leafnode for transport, no application code needed.
//
// Architecture under test:
//
//	[External NATS (spoke)] <-- leafnode --> [notif NATS (hub)]
func TestLeafnodeSubjectFlow(t *testing.T) {
	hub, spoke := startLeafnodePair(t)

	// Connect clients
	hubNC := connectClient(t, hub)
	spokeNC := connectClient(t, spoke)

	// Test 1: Messages flow from spoke (external) to hub (notif) via leafnode
	t.Run("spoke_to_hub", func(t *testing.T) {
		received := make(chan *nats.Msg, 1)

		sub, err := hubNC.Subscribe("message.received.>", func(msg *nats.Msg) {
			received <- msg
		})
		if err != nil {
			t.Fatalf("subscribe hub: %v", err)
		}
		defer sub.Unsubscribe()
		hubNC.Flush()
		time.Sleep(200 * time.Millisecond)

		payload := map[string]string{"channel": "whatsapp", "from": "5511999"}
		data, _ := json.Marshal(payload)
		if err := spokeNC.Publish("message.received.whatsapp.wa-001", data); err != nil {
			t.Fatalf("publish spoke: %v", err)
		}
		spokeNC.Flush()

		select {
		case msg := <-received:
			if msg.Subject != "message.received.whatsapp.wa-001" {
				t.Errorf("expected subject message.received.whatsapp.wa-001, got %s", msg.Subject)
			}
			var got map[string]string
			json.Unmarshal(msg.Data, &got)
			if got["channel"] != "whatsapp" {
				t.Errorf("expected channel whatsapp, got %s", got["channel"])
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for message on hub")
		}
	})

	// Test 2: Messages flow from hub (notif) to spoke (external) via leafnode
	t.Run("hub_to_spoke", func(t *testing.T) {
		received := make(chan *nats.Msg, 1)

		sub, err := spokeNC.Subscribe("message.send.>", func(msg *nats.Msg) {
			received <- msg
		})
		if err != nil {
			t.Fatalf("subscribe spoke: %v", err)
		}
		defer sub.Unsubscribe()
		spokeNC.Flush()
		time.Sleep(200 * time.Millisecond)

		payload := map[string]string{"to": "5511999", "text": "Hello!"}
		data, _ := json.Marshal(payload)
		if err := hubNC.Publish("message.send.whatsapp.wa-001", data); err != nil {
			t.Fatalf("publish hub: %v", err)
		}
		hubNC.Flush()

		select {
		case msg := <-received:
			if msg.Subject != "message.send.whatsapp.wa-001" {
				t.Errorf("expected subject message.send.whatsapp.wa-001, got %s", msg.Subject)
			}
			var got map[string]string
			json.Unmarshal(msg.Data, &got)
			if got["text"] != "Hello!" {
				t.Errorf("expected text 'Hello!', got %s", got["text"])
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for message on spoke")
		}
	})

	// Test 3: JetStream on hub captures leafnode messages
	t.Run("leafnode_to_jetstream", func(t *testing.T) {
		hubJS, err := jetstream.New(hubNC)
		if err != nil {
			t.Fatalf("jetstream: %v", err)
		}

		ctx := context.Background()
		stream, err := hubJS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
			Name:     "TEST_LEAFNODE",
			Subjects: []string{"external.>"},
			Storage:  jetstream.MemoryStorage,
		})
		if err != nil {
			t.Fatalf("create stream: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		payload := map[string]string{"source": "spoke"}
		data, _ := json.Marshal(payload)
		if err := spokeNC.Publish("external.test.event", data); err != nil {
			t.Fatalf("publish spoke: %v", err)
		}
		spokeNC.Flush()

		consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			FilterSubject: "external.>",
			DeliverPolicy: jetstream.DeliverAllPolicy,
			AckPolicy:     jetstream.AckNonePolicy,
		})
		if err != nil {
			t.Fatalf("create consumer: %v", err)
		}

		msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}

		var got bool
		for msg := range msgs.Messages() {
			if msg.Subject() == "external.test.event" {
				got = true
			}
		}
		if !got {
			t.Fatal("leafnode message did not arrive in JetStream")
		}
	})
}

// TestLeafnodeWithApplicationMapping verifies the end-to-end flow:
// external subject arrives via leafnode -> application-level subscriber
// maps it to the notif internal namespace -> JetStream captures it.
//
// This simulates what the interceptor does: subscribe to raw external
// subjects, transform/map, and republish into the events.{org}.{proj} namespace.
func TestLeafnodeWithApplicationMapping(t *testing.T) {
	hub, spoke := startLeafnodePair(t)

	hubNC := connectClient(t, hub)
	spokeNC := connectClient(t, spoke)

	hubJS, err := jetstream.New(hubNC)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	ctx := context.Background()

	// Create events stream on hub (like notif does)
	stream, err := hubJS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "NOTIF_EVENTS",
		Subjects: []string{"events.>"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	// Application-level mapping subscriber:
	// message.received.X -> events.org_default.default.omni.inbound.X
	_, err = hubNC.Subscribe("message.received.>", func(msg *nats.Msg) {
		suffix := msg.Subject[len("message.received."):]
		targetSubject := "events.org_default.default.omni.inbound." + suffix
		hubNC.Publish(targetSubject, msg.Data)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	hubNC.Flush()
	time.Sleep(200 * time.Millisecond)

	// Publish from spoke (simulating Omni)
	payload := map[string]string{"from": "wa-001", "text": "Oi"}
	data, _ := json.Marshal(payload)
	spokeNC.Publish("message.received.whatsapp.wa-001", data)
	spokeNC.Flush()

	// Verify it arrives in JetStream with the mapped subject
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject: "events.org_default.default.omni.inbound.>",
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckPolicy:     jetstream.AckNonePolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	var got bool
	for msg := range msgs.Messages() {
		if msg.Subject() == "events.org_default.default.omni.inbound.whatsapp.wa-001" {
			got = true
			var p map[string]string
			json.Unmarshal(msg.Data(), &p)
			if p["text"] != "Oi" {
				t.Errorf("unexpected payload: %s", string(msg.Data()))
			}
		}
	}
	if !got {
		t.Fatal("mapped message did not arrive in JetStream")
	}
}

// --- Test helpers ---

// startLeafnodePair starts a hub NATS server with leafnode listener and a
// spoke NATS server that connects to it via leafnode. Returns both servers.
func startLeafnodePair(t *testing.T) (hub, spoke *natsserver.Server) {
	t.Helper()

	hubOpts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		LeafNode: natsserver.LeafNodeOpts{
			Host: "127.0.0.1",
			Port: -1, // random port
		},
	}

	hub, err := natsserver.NewServer(hubOpts)
	if err != nil {
		t.Fatalf("start hub: %v", err)
	}
	hub.Start()
	t.Cleanup(hub.Shutdown)
	if !hub.ReadyForConnections(5 * time.Second) {
		t.Fatal("hub not ready")
	}

	// After Start(), the Options.LeafNode.Port is resolved to the actual port
	leafPort := hubOpts.LeafNode.Port
	leafURL, _ := url.Parse(fmt.Sprintf("nats://127.0.0.1:%d", leafPort))

	spokeOpts := &natsserver.Options{
		Host:  "127.0.0.1",
		Port:  -1,
		NoLog: true,
		LeafNode: natsserver.LeafNodeOpts{
			Remotes: []*natsserver.RemoteLeafOpts{
				{URLs: []*url.URL{leafURL}},
			},
		},
	}

	spoke, err = natsserver.NewServer(spokeOpts)
	if err != nil {
		t.Fatalf("start spoke: %v", err)
	}
	spoke.Start()
	t.Cleanup(spoke.Shutdown)
	if !spoke.ReadyForConnections(5 * time.Second) {
		t.Fatal("spoke not ready")
	}

	// Wait for leafnode connection
	waitForLeafnode(t, hub, 5*time.Second)

	return hub, spoke
}

// connectClient connects a NATS client to the given server.
func connectClient(t *testing.T, srv *natsserver.Server) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	return nc
}

// waitForLeafnode waits until the server has at least one leafnode connection.
func waitForLeafnode(t *testing.T, srv *natsserver.Server, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if srv.NumLeafNodes() > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for leafnode connection")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

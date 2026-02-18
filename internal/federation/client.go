package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Event represents a notif event received via WebSocket.
type Event struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client is a Go-native notif client (WS subscribe + HTTP emit).
type Client struct {
	url, apiKey, wsURL, emitURL string
	http                        *http.Client
	logger                      *slog.Logger
	initialBackoff, maxBackoff  time.Duration
}

func NewClient(url, apiKey string, logger *slog.Logger) *Client {
	if logger == nil { logger = slog.Default() }
	ws := strings.Replace(strings.Replace(url, "https://", "wss://", 1), "http://", "ws://", 1)
	return &Client{url: url, apiKey: apiKey, wsURL: ws + "/ws", emitURL: url + "/api/v1/emit",
		http: &http.Client{Timeout: 10 * time.Second}, logger: logger,
		initialBackoff: time.Second, maxBackoff: 30 * time.Second}
}

// Subscribe connects via WebSocket, returns events channel. Reconnects on disconnect.
func (c *Client) Subscribe(ctx context.Context, topics []string) (<-chan *Event, error) {
	conn, err := c.connect(ctx, topics)
	if err != nil { return nil, fmt.Errorf("initial connect: %w", err) }
	ch := make(chan *Event, 64)
	go c.readLoop(ctx, conn, topics, ch)
	return ch, nil
}

func (c *Client) connect(ctx context.Context, topics []string) (*websocket.Conn, error) {
	hdr := http.Header{"Authorization": {"Bearer " + c.apiKey}}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.wsURL, hdr)
	if err != nil { return nil, fmt.Errorf("ws dial: %w", err) }
	if err := conn.WriteJSON(map[string]any{"action": "subscribe", "topics": topics,
		"options": map[string]any{"auto_ack": true, "from": "latest"}}); err != nil {
		conn.Close(); return nil, fmt.Errorf("send subscribe: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var msg struct{ Type string `json:"type"` }
	if err := conn.ReadJSON(&msg); err != nil { conn.Close(); return nil, fmt.Errorf("read subscribed: %w", err) }
	if msg.Type != "subscribed" { conn.Close(); return nil, fmt.Errorf("expected subscribed, got %s", msg.Type) }
	conn.SetReadDeadline(time.Time{})
	c.logger.Info("federation: subscribed", "topics", topics, "url", c.url)
	return conn, nil
}

type wsMsg struct{ d []byte; e error }

func startReader(conn *websocket.Conn, ch chan<- wsMsg) {
	go func() {
		for { _, d, e := conn.ReadMessage(); ch <- wsMsg{d, e}; if e != nil { return } }
	}()
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn, topics []string, ch chan<- *Event) {
	defer close(ch)
	ping := time.NewTicker(30 * time.Second); defer ping.Stop()
	rc := make(chan wsMsg, 1); startReader(conn, rc)
	bo := c.initialBackoff
	for {
		select {
		case <-ctx.Done(): conn.Close(); return
		case <-ping.C: conn.WriteJSON(map[string]string{"action": "ping"})
		case m := <-rc:
			if m.e != nil {
				c.logger.Warn("federation: ws error, reconnecting", "error", m.e); conn.Close()
				for {
					select { case <-ctx.Done(): return; case <-time.After(bo): }
					var err error
					if conn, err = c.connect(ctx, topics); err != nil { bo = min(bo*2, c.maxBackoff); continue }
					bo = c.initialBackoff; startReader(conn, rc); break
				}
				continue
			}
			var evt Event
			if json.Unmarshal(m.d, &evt) != nil || evt.Type != "event" { continue }
			select { case ch <- &evt: case <-ctx.Done(): conn.Close(); return }
		}
	}
}

// Emit sends an event via HTTP POST /api/v1/emit. Retries up to 3x.
func (c *Client) Emit(ctx context.Context, topic string, data json.RawMessage) error {
	body, _ := json.Marshal(map[string]any{"topic": topic, "data": data})
	var lastErr error
	for i := range 3 {
		if i > 0 { select { case <-ctx.Done(): return ctx.Err(); case <-time.After(time.Duration(i) * time.Second): } }
		req, _ := http.NewRequestWithContext(ctx, "POST", c.emitURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		resp, err := c.http.Do(req)
		if err != nil { lastErr = err; continue }
		io.Copy(io.Discard, resp.Body); resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
		lastErr = fmt.Errorf("emit: status %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 { return lastErr } // permanent error, don't retry
	}
	return lastErr
}

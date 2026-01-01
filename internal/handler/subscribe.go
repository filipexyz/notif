package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/filipexyz/notif/internal/middleware"
	"github.com/filipexyz/notif/internal/nats"
	"github.com/filipexyz/notif/internal/websocket"
	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add proper origin validation in production
		return true
	},
}

// SubscribeHandler handles WebSocket subscriptions.
type SubscribeHandler struct {
	hub         *websocket.Hub
	consumerMgr *nats.ConsumerManager
}

// NewSubscribeHandler creates a new SubscribeHandler.
func NewSubscribeHandler(hub *websocket.Hub, consumerMgr *nats.ConsumerManager) *SubscribeHandler {
	return &SubscribeHandler{
		hub:         hub,
		consumerMgr: consumerMgr,
	}
}

// Subscribe upgrades HTTP to WebSocket and handles subscriptions.
func (h *SubscribeHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	apiKey := middleware.GetAPIKey(r.Context())
	apiKeyID := ""
	env := "live"
	if apiKey != nil {
		apiKeyID = uuid.UUID(apiKey.ID.Bytes).String()
		env = apiKey.Environment
	}

	client := websocket.NewClient(h.hub, conn, apiKeyID, env)
	h.hub.Register(client)

	// Start read/write pumps with a fresh context (not the HTTP request context)
	go client.WritePump()
	go client.ReadPump(context.Background(), h.consumerMgr)
}

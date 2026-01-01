package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"github.com/filipexyz/notif/internal/db"
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
	hub          *websocket.Hub
	consumerMgr  *nats.ConsumerManager
	dlqPublisher *nats.DLQPublisher
	queries      *db.Queries
}

// NewSubscribeHandler creates a new SubscribeHandler.
func NewSubscribeHandler(hub *websocket.Hub, consumerMgr *nats.ConsumerManager, dlqPublisher *nats.DLQPublisher, queries *db.Queries) *SubscribeHandler {
	return &SubscribeHandler{
		hub:          hub,
		consumerMgr:  consumerMgr,
		dlqPublisher: dlqPublisher,
		queries:      queries,
	}
}

// generateClientID creates a unique client identifier.
func generateClientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ws_" + hex.EncodeToString(b)
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
	if apiKey != nil {
		apiKeyID = uuid.UUID(apiKey.ID.Bytes).String()
	}

	// Get org_id from auth context for multi-tenant isolation
	authCtx := middleware.GetAuthContext(r.Context())
	orgID := ""
	if authCtx != nil {
		orgID = authCtx.OrgID
	}

	clientID := generateClientID()
	client := websocket.NewClient(h.hub, conn, apiKeyID, orgID, h.dlqPublisher, h.queries, clientID)
	h.hub.Register(client)

	slog.Info("websocket client connected", "client_id", clientID)

	// Start read/write pumps with a fresh context (not the HTTP request context)
	go client.WritePump()
	go client.ReadPump(context.Background(), h.consumerMgr)
}

package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestWSGatewaySubscriptionFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "testsecret12345678"
	gateway := NewWSGateway(secret)
	go gateway.Run()

	r := gin.New()
	r.GET("/ws", gateway.HandleConnection)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Connect utilizing gorilla/websocket dialer
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish WS dialer handshake: %v", err)
	}
	defer ws.Close()

	// Send Ping action
	pingMsg := WSMessage{Action: "ping"}
	payload, _ := json.Marshal(pingMsg)
	_ = ws.WriteMessage(websocket.TextMessage, payload)

	_, resp, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read pong response: %v", err)
	}

	if !strings.Contains(string(resp), "pong") {
		t.Errorf("Expected pong response, got %s", string(resp))
	}

	// Send Subscribe action
	subMsg := WSMessage{Action: "subscribe", Channel: "market:ticker"}
	subPayload, _ := json.Marshal(subMsg)
	_ = ws.WriteMessage(websocket.TextMessage, subPayload)

	_, subResp, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read subscribe response: %v", err)
	}

	if !strings.Contains(string(subResp), "subscribed") {
		t.Errorf("Expected subscribed response, got %s", string(subResp))
	}

	// Broadcast tick message to channel
	gateway.BroadcastToChannel("market:ticker", []byte(`{"ticker":"BTC-USDT","price":52000.5}`))

	// Read message from broadcast channel
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, tickMsg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read broadcast tick message: %v", err)
	}

	if !strings.Contains(string(tickMsg), "52000.5") {
		t.Errorf("Expected price '52000.5' in broadcast payload, got %s", string(tickMsg))
	}
}

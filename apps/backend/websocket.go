package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"velyxora/packages/security"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the exchange gateway pool
	},
}

type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	userID    string
	channels  map[string]bool
	mu        sync.Mutex
	unhealthy bool
}

type WSMessage struct {
	Action  string          `json:"action"`  // "subscribe", "unsubscribe", "ping"
	Channel string          `json:"channel"` // e.g. "market:ticker", "market:trades"
	Token   string          `json:"token"`   // JWT for secure user stream authentication
	Payload json.RawMessage `json:"payload"`
}

type WSGateway struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	jwtSecret  string
}

func NewWSGateway(jwtSecret string) *WSGateway {
	return &WSGateway{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		jwtSecret:  jwtSecret,
	}
}

func (g *WSGateway) Run() {
	for {
		select {
		case client := <-g.register:
			g.mu.Lock()
			g.clients[client] = true
			g.mu.Unlock()
		case client := <-g.unregister:
			g.mu.Lock()
			if _, ok := g.clients[client]; ok {
				delete(g.clients, client)
				close(client.send)
			}
			g.mu.Unlock()
		}
	}
}

func (g *WSGateway) BroadcastToChannel(channel string, message []byte) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for client := range g.clients {
		client.mu.Lock()
		if client.channels[channel] && !client.unhealthy {
			select {
			case client.send <- message:
			default:
				// Outbound queue is full! Client is too slow to process high-frequency stream.
				// Mark unhealthy and disconnect safely.
				client.unhealthy = true
				go func(c *Client) {
					c.mu.Lock()
					defer c.mu.Unlock()
					_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Slow Consumer Protection Triggered"))
					_ = c.conn.Close()
				}(client)
			}
		}
		client.mu.Unlock()
	}
}

func (g *WSGateway) HandleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}

	g.register <- client

	// Start read/write loops
	go g.writeLoop(client)
	go g.readLoop(client)
}

func (g *WSGateway) readLoop(c *Client) {
	defer func() {
		g.unregister <- c
		c.conn.Close()
	}()

	// Connection lifecycle timeout and heartbeat limits
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Action {
		case "ping":
			c.send <- []byte(`{"event":"pong"}`)
		case "subscribe":
			c.mu.Lock()
			// Handle authentication dynamically for private streams
			if strings.HasPrefix(wsMsg.Channel, "private:") {
				claims, err := security.ValidateJWT(wsMsg.Token, g.jwtSecret)
				if err != nil {
					c.mu.Unlock()
					c.send <- []byte(`{"event":"error","message":"unauthorized"}`)
					continue
				}
				c.userID = claims.UserID
			}
			c.channels[wsMsg.Channel] = true
			c.mu.Unlock()
			c.send <- []byte(`{"event":"subscribed","channel":"` + wsMsg.Channel + `"}`)
		case "unsubscribe":
			c.mu.Lock()
			delete(c.channels, wsMsg.Channel)
			c.mu.Unlock()
			c.send <- []byte(`{"event":"unsubscribed","channel":"` + wsMsg.Channel + `"}`)
		}
	}
}

func (g *WSGateway) writeLoop(c *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.mu.Lock()
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.mu.Unlock()
				return
			}
			c.mu.Lock()
			_ = c.conn.WriteMessage(websocket.TextMessage, msg)
			c.mu.Unlock()
		case <-ticker.C:
			c.mu.Lock()
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

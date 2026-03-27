package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Message is a WebSocket message envelope.
type Message struct {
	Type      string      `json:"type"`      // metrics, alert
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Hub manages WebSocket connections and broadcasts messages.
type Hub struct {
	logger  *slog.Logger
	mu      sync.RWMutex
	clients map[*Client]bool
}

// Client wraps a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger:  logger,
		clients: make(map[*Client]bool),
	}
}

// HandleConnect upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) HandleConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 64),
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	h.logger.Info("ws client connected", "remote", conn.RemoteAddr(), "total", h.ClientCount())

	go client.writePump()
	go client.readPump()
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("ws marshal failed", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			// Client buffer full, close
			go h.removeClient(client)
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		c.conn.Close()
	}
	h.mu.Unlock()
	h.logger.Info("ws client disconnected", "total", h.ClientCount())
}

func (c *Client) readPump() {
	defer c.hub.removeClient(c)

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	pingTicker := time.NewTicker(30 * time.Second)
	defer func() {
		pingTicker.Stop()
		c.hub.removeClient(c)
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

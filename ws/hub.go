package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dadidudidap/url-shortener/database"
)

const (
	writeWait  = 5 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
	maxMessage = 1024
)

var upgrader = websocket.Upgrader{
	CheckOrigin: sameOrigin,
}

// sameOrigin keeps other sites from opening sockets through a visitor's browser.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser client
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

type client struct {
	conn *websocket.Conn
	code string
	// Gorilla permits one writer at a time; Broadcast and the ping ticker share this lock.
	mu sync.Mutex
}

func (c *client) write(messageType int, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(messageType, payload)
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{}
}

var Manager = &Hub{
	clients: make(map[string]map[*client]struct{}),
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, shortCode string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	c := &client{conn: conn, code: shortCode}

	h.mu.Lock()
	if h.clients[shortCode] == nil {
		h.clients[shortCode] = make(map[*client]struct{})
	}
	h.clients[shortCode][c] = struct{}{}
	h.mu.Unlock()

	log.Printf("New WebSocket connection for %s", shortCode)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := c.write(websocket.PingMessage, nil); err != nil {
					// Unblock the reader so the connection is cleaned up now.
					conn.Close()
					return
				}
			}
		}
	}()

	defer func() {
		close(done)
		h.remove(c)
		conn.Close()
		log.Printf("WebSocket disconnected for %s", shortCode)
	}()

	conn.SetReadLimit(maxMessage)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.clients[c.code]
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.clients, c.code)
	}
}

func (h *Hub) Broadcast(shortCode string, clicks int32) {
	message, err := json.Marshal(map[string]any{
		"short_code": shortCode,
		"clicks":     clicks,
	})
	if err != nil {
		return
	}

	// Snapshot under the lock: ranging the live map races with remove's delete.
	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients[shortCode]))
	for c := range h.clients[shortCode] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if err := c.write(websocket.TextMessage, message); err != nil {
			// Bounded by writeWait, so a dead viewer cannot stall the click worker.
			c.conn.Close()
		}
	}
}

// CloseAll disconnects viewers on shutdown; http.Server.Shutdown ignores upgraded connections.
func (h *Hub) CloseAll() {
	h.mu.RLock()
	targets := make([]*client, 0)
	for _, conns := range h.clients {
		for c := range conns {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		_ = c.write(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
		c.conn.Close()
	}
}

func (h *Hub) ListenRedis() {
	pubsub := database.RedisClient.Subscribe(context.Background(), "ws_broadcasts")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("WebSocket Hub listening to Redis Pub/Sub...")

	for msg := range ch {
		// Split "abc123:50" into shortCode and clicks
		parts := strings.Split(msg.Payload, ":")
		if len(parts) == 2 {
			shortCode := parts[0]
			clicks, _ := strconv.Atoi(parts[1])

			// Push the data to any connected browser!
			h.Broadcast(shortCode, int32(clicks))
		}
	}
}

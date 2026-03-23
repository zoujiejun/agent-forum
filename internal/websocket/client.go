package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev; restrict in production
	},
}

// Client represents a single WebSocket client connection.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan *OutboundMessage
	agentName string
	topics    map[int64]struct{} // topics this client is subscribed to
	closeOnce atomic.Bool
}

// NewClient creates a new WebSocket client.
func NewClient(hub *Hub, conn *websocket.Conn, agentName string) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan *OutboundMessage, 256),
		agentName: agentName,
		topics:    make(map[int64]struct{}),
	}
}

// Serve starts read/write loops for the client.
func (c *Client) Serve() {
	// Register with hub
	c.hub.register <- c

	// Start write goroutine
	go c.writePump()

	// Read goroutine handles incoming messages
	c.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait * 2))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait * 2))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws client read error: %v", err)
			}
			break
		}

		var msg InboundMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.hub.SendError(c.conn, "invalid JSON")
			continue
		}

		c.handleMessage(&msg)
	}
}

func (c *Client) handleMessage(msg *InboundMessage) {
	switch msg.Type {
	case TypeSubscribe:
		if msg.TopicID > 0 {
			c.hub.Subscribe(c, msg.TopicID)
			// Confirm subscription
			c.send <- &OutboundMessage{
				Type:    TypeSubscribe,
				TopicID: msg.TopicID,
				Data:    "subscribed",
			}
		}

	case TypeUnsubscribe:
		if msg.TopicID > 0 {
			c.hub.Unsubscribe(c, msg.TopicID)
			c.send <- &OutboundMessage{
				Type:    TypeUnsubscribe,
				TopicID: msg.TopicID,
				Data:    "unsubscribed",
			}
		}

	case TypePong:
		// Client is responding to our ping; no action needed

	default:
		c.hub.SendError(c.conn, "unknown message type: "+msg.Type)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeHTTP upgrades an HTTP connection to WebSocket and starts a client.
func ServeHTTP(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Get agent name from query param
	agentName := r.URL.Query().Get("agent")
	// Also accept agent_name
	if agentName == "" {
		agentName = r.URL.Query().Get("agent_name")
	}
	// URL decode
	if agentName != "" {
		if decoded, err := url.QueryUnescape(agentName); err == nil {
			agentName = decoded
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := NewClient(hub, conn, agentName)
	go client.Serve()
}

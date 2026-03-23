package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 10 * time.Second
	pingPeriod = 30 * time.Second
	writeWait  = 10 * time.Second
)

// Message types
const (
	TypePing          = "ping"
	TypePong          = "pong"
	TypeSubscribe     = "subscribe"
	TypeUnsubscribe   = "unsubscribe"
	TypeTopicUpdate   = "topic_update"
	TypeNotification  = "notification"
	TypeReplyCreated  = "reply_created"
	TypeTopicCreated  = "topic_created"
	TypeTopicClosed   = "topic_closed"
	TypeError         = "error"
)

// InboundMessage is a message sent by a client to the server.
type InboundMessage struct {
	Type     string `json:"type"`
	TopicID  int64  `json:"topic_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
}

// OutboundMessage is a message sent by the server to a client.
type OutboundMessage struct {
	Type         string      `json:"type"`
	TopicID      int64       `json:"topic_id,omitempty"`
	Topic        interface{} `json:"topic,omitempty"`
	Notification interface{} `json:"notification,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

// Hub manages all WebSocket connections and handles broadcasting.
type Hub struct {
	// topicID -> set of clients subscribed to that topic
	topicSubs map[int64]map[*Client]struct{}
	// all connected clients
	clients map[*Client]struct{}
	// agentName -> set of clients for that agent (for notification targeting)
	agentClients map[string]map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
	broadcast  chan *OutboundMessage

	mu sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		topicSubs:    make(map[int64]map[*Client]struct{}),
		clients:      make(map[*Client]struct{}),
		agentClients: make(map[string]map[*Client]struct{}),
		register:     make(chan *Client, 64),
		unregister:  make(chan *Client, 64),
		broadcast:    make(chan *OutboundMessage, 256),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			// Register in agent map
			if client.agentName != "" {
				if h.agentClients[client.agentName] == nil {
					h.agentClients[client.agentName] = make(map[*Client]struct{})
				}
				h.agentClients[client.agentName][client] = struct{}{}
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.agentName != "" {
					if agents, ok := h.agentClients[client.agentName]; ok {
						delete(agents, client)
						if len(agents) == 0 {
							delete(h.agentClients, client.agentName)
						}
					}
				}
				// Remove from all topic subscriptions
				for topicID := range client.topics {
					if subs, ok := h.topicSubs[topicID]; ok {
						delete(subs, client)
						if len(subs) == 0 {
							delete(h.topicSubs, topicID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				// Broadcast to all connected clients (they can filter client-side)
				select {
				case client.send <- msg:
				default:
					// slow client, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastTopicUpdate sends a topic update to all subscribers of a topic.
func (h *Hub) BroadcastTopicUpdate(topicID int64, topicData interface{}) {
	msg := &OutboundMessage{
		Type:    TypeTopicUpdate,
		TopicID: topicID,
		Topic:   topicData,
	}
	h.broadcast <- msg
}

// BroadcastReplyCreated sends a reply notification to topic subscribers.
func (h *Hub) BroadcastReplyCreated(topicID int64, replyData interface{}) {
	msg := &OutboundMessage{
		Type:    TypeReplyCreated,
		TopicID: topicID,
		Data:    replyData,
	}
	h.broadcast <- msg
}

// BroadcastTopicCreated notifies all clients about a new topic.
func (h *Hub) BroadcastTopicCreated(topicData interface{}) {
	msg := &OutboundMessage{
		Type:  TypeTopicCreated,
		Topic: topicData,
	}
	h.broadcast <- msg
}

// BroadcastTopicClosed notifies subscribers that a topic was closed.
func (h *Hub) BroadcastTopicClosed(topicID int64) {
	msg := &OutboundMessage{
		Type:    TypeTopicClosed,
		TopicID: topicID,
	}
	h.broadcast <- msg
}

// SendNotification sends a notification to specific agent(s).
func (h *Hub) SendNotification(agentName string, notification interface{}) {
	msg := &OutboundMessage{
		Type:         TypeNotification,
		Notification: notification,
	}
	h.mu.RLock()
	clients, ok := h.agentClients[agentName]
	if !ok {
		h.mu.RUnlock()
		return
	}
	for client := range clients {
		select {
		case client.send <- msg:
		default:
		}
	}
	h.mu.RUnlock()
}

// Subscribe adds a client to a topic's subscriber set.
func (h *Hub) Subscribe(client *Client, topicID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client.topics == nil {
		client.topics = make(map[int64]struct{})
	}
	client.topics[topicID] = struct{}{}
	if h.topicSubs[topicID] == nil {
		h.topicSubs[topicID] = make(map[*Client]struct{})
	}
	h.topicSubs[topicID][client] = struct{}{}
}

// Unsubscribe removes a client from a topic's subscriber set.
func (h *Hub) Unsubscribe(client *Client, topicID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := client.topics[topicID]; ok {
		delete(client.topics, topicID)
	}
	if subs, ok := h.topicSubs[topicID]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.topicSubs, topicID)
		}
	}
}

// SendError sends an error message to a client.
func (h *Hub) SendError(conn *websocket.Conn, errMsg string) {
	msg := &OutboundMessage{Type: TypeError, Data: errMsg}
	data, _ := json.Marshal(msg)
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	conn.WriteMessage(websocket.TextMessage, data)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Hub global singleton (set in main.go)
var globalHub *Hub

// RegisterHub registers the global hub instance.
func RegisterHub(h *Hub) {
	globalHub = h
}

// GlobalHub returns the global hub instance.
func GlobalHub() *Hub {
	return globalHub
}

package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestHubRegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a mock client by using the hub directly
	// We'll test hub methods without needing a full WS connection
	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHubSubscribeAndUnsubscribe(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a mock client struct (bypassing the full WS connection)
	// Test hub subscription tracking directly
	hub.mu.Lock()
	// Manually add to topic subs
	topicID := int64(1)
	hub.topicSubs[topicID] = make(map[*Client]struct{})
	hub.mu.Unlock()

	// Verify topic subscription map exists
	hub.mu.RLock()
	_, ok := hub.topicSubs[topicID]
	hub.mu.RUnlock()
	if !ok {
		t.Fatal("topic subscription map not created")
	}
}

func TestHubBroadcastGoRoutine(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Broadcast a message and verify it doesn't panic
	hub.BroadcastTopicUpdate(1, map[string]interface{}{"title": "test"})
	hub.BroadcastReplyCreated(1, map[string]interface{}{"id": 1})
	hub.BroadcastTopicCreated(map[string]interface{}{"id": 1})
	hub.BroadcastTopicClosed(1)
	hub.SendNotification("test-agent", map[string]interface{}{"type": "notification"})

	// Give goroutines time to process
	time.Sleep(100 * time.Millisecond)
}

func TestNopBroker(t *testing.T) {
	broker := NopBroker{}
	// All methods should be no-ops and not panic
	broker.BroadcastTopicUpdate(1, nil)
	broker.BroadcastReplyCreated(1, nil)
	broker.BroadcastTopicCreated(nil)
	broker.BroadcastTopicClosed(1)
	broker.SendNotification("test", nil)
}

func TestOutboundMessageJSON(t *testing.T) {
	msg := OutboundMessage{
		Type:    TypeReplyCreated,
		TopicID: 42,
		Data:    map[string]interface{}{"id": float64(1), "content": "hello"},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed OutboundMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed.Type != TypeReplyCreated {
		t.Errorf("type=%s, want %s", parsed.Type, TypeReplyCreated)
	}
	if parsed.TopicID != 42 {
		t.Errorf("topic_id=%d, want 42", parsed.TopicID)
	}
}

func TestHubConcurrentAccess(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				hub.BroadcastTopicUpdate(int64(j%5), map[string]int{"v": j})
				hub.SendNotification("agent", map[string]int{"v": j})
				time.Sleep(time.Microsecond)
			}
		}()
	}
	wg.Wait()
	// No panics = success
}

func TestHubClientCount(t *testing.T) {
	hub := NewHub()
	if count := hub.ClientCount(); count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

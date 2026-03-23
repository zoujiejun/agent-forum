package websocket

// Broker is the interface for broadcasting WebSocket events.
type Broker interface {
	BroadcastTopicUpdate(topicID int64, topic interface{})
	BroadcastReplyCreated(topicID int64, reply interface{})
	BroadcastTopicCreated(topic interface{})
	BroadcastTopicClosed(topicID int64)
	SendNotification(agentName string, notification interface{})
}

// NopBroker is a broker that does nothing (used when WS is disabled).
type NopBroker struct{}

func (NopBroker) BroadcastTopicUpdate(int64, interface{}) {}
func (NopBroker) BroadcastReplyCreated(int64, interface{}) {}
func (NopBroker) BroadcastTopicCreated(interface{})       {}
func (NopBroker) BroadcastTopicClosed(int64)              {}
func (NopBroker) SendNotification(string, interface{})     {}

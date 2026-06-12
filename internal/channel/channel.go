package channel

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Message represents a published message with channel, payload, and timestamp.
type Message struct {
	Channel string      `json:"channel"`
	Payload interface{} `json:"payload"`
	TS      int64       `json:"ts"`
}

// Channel manages a single pub/sub topic with last-value cache.
// All methods are thread-safe via sync.RWMutex.
type Channel struct {
	name        string
	subscribers map[*websocket.Conn]struct{}
	lastValue   *Message
	mu          sync.RWMutex
}

// New creates a new named channel.
func New(name string) *Channel {
	return &Channel{
		name:        name,
		subscribers: make(map[*websocket.Conn]struct{}),
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return c.name
}

// AddSubscriber adds a WebSocket connection to this channel's subscriber list.
func (c *Channel) AddSubscriber(conn *websocket.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers[conn] = struct{}{}
}

// RemoveSubscriber removes a WebSocket connection from this channel's subscriber list.
func (c *Channel) RemoveSubscriber(conn *websocket.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, conn)
}

// Subscribers returns a snapshot of all current subscriber connections.
func (c *Channel) Subscribers() []*websocket.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*websocket.Conn, 0, len(c.subscribers))
	for conn := range c.subscribers {
		result = append(result, conn)
	}
	return result
}

// SetLastValue stores the most recent message for this channel.
func (c *Channel) SetLastValue(msg *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastValue = msg
}

// LastValue returns the most recent message, or nil if none has been published.
func (c *Channel) LastValue() *Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastValue
}

// SubscriberCount returns the number of active subscribers.
func (c *Channel) SubscriberCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.subscribers)
}

// HasSubscribers returns true if at least one subscriber is connected.
func (c *Channel) HasSubscribers() bool {
	return c.SubscriberCount() > 0
}

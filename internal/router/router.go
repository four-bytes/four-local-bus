package router

import (
	"strings"
	"sync"
	"time"

	"github.com/four-bytes/four-opencode-plugin-bus/internal/channel"
	"github.com/gorilla/websocket"
)

// Router manages channel subscriptions and message delivery.
// All map access is protected by sync.RWMutex.
type Router struct {
	mu sync.RWMutex

	// channels maps channel name → channel instance
	channels map[string]*channel.Channel

	// subs maps subscription pattern → set of connections
	subs map[string]map[*websocket.Conn]struct{}

	// connPatterns maps connection → set of patterns it subscribes to (for cleanup)
	connPatterns map[*websocket.Conn]map[string]struct{}

	// connWriters maps connection → mutex for serializing WebSocket writes
	connWriters map[*websocket.Conn]*sync.Mutex
}

// New creates a new Router.
func New() *Router {
	return &Router{
		channels:     make(map[string]*channel.Channel),
		subs:         make(map[string]map[*websocket.Conn]struct{}),
		connPatterns: make(map[*websocket.Conn]map[string]struct{}),
		connWriters:  make(map[*websocket.Conn]*sync.Mutex),
	}
}

// Subscribe adds a connection to a channel pattern subscription.
// Last-value cache messages for matching channels are delivered immediately.
//
// The pattern supports '+' as a wildcard matching exactly one path segment.
// Pattern is normalized to lowercase with trimmed whitespace.
func (r *Router) Subscribe(pattern string, conn *websocket.Conn) {
	pattern = strings.TrimSpace(strings.ToLower(pattern))

	r.mu.Lock()

	// Register in subs[pattern]
	if r.subs[pattern] == nil {
		r.subs[pattern] = make(map[*websocket.Conn]struct{})
	}
	r.subs[pattern][conn] = struct{}{}

	// Register in connPatterns
	if r.connPatterns[conn] == nil {
		r.connPatterns[conn] = make(map[string]struct{})
	}
	r.connPatterns[conn][pattern] = struct{}{}

	// Ensure writer mutex exists
	if _, ok := r.connWriters[conn]; !ok {
		r.connWriters[conn] = &sync.Mutex{}
	}
	writerMu := r.connWriters[conn]

	// Collect last-value messages for matching channels
	var backlog []*channel.Message
	for chName, ch := range r.channels {
		if matchPattern(pattern, chName) {
			if msg := ch.LastValue(); msg != nil {
				backlog = append(backlog, msg)
			}
		}
	}

	r.mu.Unlock()

	// Deliver backlog messages (outside lock, with per-connection mutex)
	for _, msg := range backlog {
		writerMu.Lock()
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = conn.WriteJSON(msg) // best-effort delivery
		writerMu.Unlock()
	}
}

// Unsubscribe removes ALL subscriptions for the given connection.
func (r *Router) Unsubscribe(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	patterns, ok := r.connPatterns[conn]
	if !ok {
		return
	}

	// Remove connection from every pattern's subscriber list
	for pattern := range patterns {
		if conns, ok := r.subs[pattern]; ok {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(r.subs, pattern)
			}
		}
	}

	// Clean up connection tracking
	delete(r.connPatterns, conn)
	delete(r.connWriters, conn)
}

// Publish delivers a message to all subscribers whose patterns match the channel.
// The channel name is normalized (lowercase, trimmed).
// If the channel does not exist yet, it is created.
// The last-value cache is updated before delivery.
func (r *Router) Publish(channelName string, payload interface{}) {
	channelName = strings.TrimSpace(strings.ToLower(channelName))

	msg := &channel.Message{
		Channel: channelName,
		Payload: payload,
		TS:      time.Now().UnixMilli(),
	}

	// Lock: create/get channel, set last value, collect subscribers
	r.mu.Lock()

	// Get or create channel
	ch, ok := r.channels[channelName]
	if !ok {
		ch = channel.New(channelName)
		r.channels[channelName] = ch
	}
	ch.SetLastValue(msg)

	// Collect matching (conn, writerMu) pairs
	type connTarget struct {
		conn *websocket.Conn
		mu   *sync.Mutex
	}
	var targets []connTarget

	for pattern, conns := range r.subs {
		if matchPattern(pattern, channelName) {
			for conn := range conns {
				if mu, ok := r.connWriters[conn]; ok {
					targets = append(targets, connTarget{conn, mu})
				}
			}
		}
	}

	r.mu.Unlock()

	// Deliver to all matching subscribers (outside lock, serialized per-connection)
	for _, t := range targets {
		t.mu.Lock()
		t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := t.conn.WriteJSON(msg); err != nil {
			// Write failed — connection may be dead.
			// The read loop in server.go will handle cleanup on next read error.
		}
		t.mu.Unlock()
	}
}

// matchPattern checks if a subscription pattern matches a channel name.
// '+' matches exactly one path segment (e.g., "tbg/+/status" matches "tbg/ses_abc/status").
// Both pattern and channelName must already be normalized (lowercase, trimmed).
func matchPattern(pattern, channelName string) bool {
	patternSegs := strings.Split(pattern, "/")
	channelSegs := strings.Split(channelName, "/")

	if len(patternSegs) != len(channelSegs) {
		return false
	}

	for i, seg := range patternSegs {
		if seg != "+" && seg != channelSegs[i] {
			return false
		}
	}
	return true
}

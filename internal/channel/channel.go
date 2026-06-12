package channel

// Channel is a pub/sub topic with last-value cache.
type Channel struct {
	name string
}

// New creates a new channel.
func New(name string) *Channel {
	return &Channel{name: name}
}

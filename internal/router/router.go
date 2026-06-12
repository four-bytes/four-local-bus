package router

// Router manages channel subscriptions and message delivery.
type Router struct {
	channels map[string]interface{}
}

// New creates a new router.
func New() *Router {
	return &Router{channels: make(map[string]interface{})}
}

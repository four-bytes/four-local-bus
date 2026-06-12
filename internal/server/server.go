package server

// Server manages the HTTP and WebSocket endpoints for the plugin bus.
type Server struct {
	port int
}

// New creates a new bus server.
func New(port int) *Server {
	return &Server{port: port}
}

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/four-bytes/four-opencode-plugin-bus/internal/discovery"
	"github.com/four-bytes/four-opencode-plugin-bus/internal/router"
	"github.com/four-bytes/four-opencode-plugin-bus/internal/server"
)

func main() {
	// Create router and server
	r := router.New()
	srv := server.New(r)

	// Start HTTP+WebSocket server on random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Write port to discovery file for plugin clients
	if err := discovery.WritePortFile(port); err != nil {
		log.Printf("WARNING: failed to write discovery file: %v", err)
	}

	fmt.Printf("four-opencode-bus listening on port %d\n", port)

	// Graceful shutdown on SIGINT/SIGTERM or idle timeout
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\nshutting down...")
	case <-srv.IdleDone():
		fmt.Println("\nidle timeout — shutting down...")
	}

	if err := discovery.CleanupPortFile(); err != nil {
		log.Printf("WARNING: failed to cleanup discovery file: %v", err)
	}
	if err := httpSrv.Close(); err != nil {
		log.Printf("WARNING: failed to close HTTP server: %v", err)
	}
}

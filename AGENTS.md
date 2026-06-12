# AGENTS.md — four-opencode-plugin-bus

## Project
Go-based local message bus for opencode plugins. Single binary providing pub/sub IPC between server plugins and TUI components via HTTP + WebSocket.

## Tech Stack
- **Language:** Go 1.22+
- **Dependencies:** stdlib + gorilla/websocket (WebSocket upgrade)
- **Build:** CGO_ENABLED=0, static binary
- **License:** Apache-2.0

## Structure
```
cmd/bus/main.go            — Entry point
internal/
├── server/server.go       — HTTP+WS server setup
├── router/router.go       — Channel subscription routing
├── channel/channel.go     — Pub/sub with last-value cache
└── discovery/discovery.go — Port file management
```

## Conventions
- Go stdlib only — no external dependencies
- Channel naming: `{plugin}/{sessionID?}/{topic}`
- Wildcard: `+` matches single segment (e.g., `tbg/+/status`)
- Port file: `~/.cache/opencode/plugin-bus/port.json`
- Cleanup on SIGTERM/SIGINT

## Commands
| Command | Description |
|---------|-------------|
| `go build ./cmd/bus/` | Build binary |
| `go test ./...` | Run tests |
| `go run ./cmd/bus/` | Run bus server |

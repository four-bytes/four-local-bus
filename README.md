# four-opencode-plugin-bus

Go-based local message bus for [opencode](https://github.com/sst/opencode) plugins. Provides universal IPC (Inter-Process Communication) between server-side plugins and TUI components — bidirectional pub/sub over HTTP + WebSocket.

## What It Does

- **Single binary, single port** — one bus process handles all plugin communication
- **Channel-based pub/sub** — plugins publish to named channels, TUI components subscribe
- **Bidirectional** — server plugins can push to TUI, TUI can publish back to server
- **Last-value cache** — late subscribers receive the most recent message
- **Port discovery** — writes port to `~/.cache/opencode/plugin-bus/port.json` on startup
- **Caddy-like** — minimal config, auto-start, stdlib-only

## Architecture

```
Server Plugins ──HTTP POST──┐
                            ├── Plugin Bus (Go) ──WebSocket── TUI Components
TUI Components ──WebSocket──┘
```

## Quick Start

```bash
go install github.com/four-bytes/four-opencode-plugin-bus/cmd/bus@latest
four-opencode-bus
```

## Protocol

### Publish (HTTP POST)
```
POST /publish
{"channel": "tbg/ses_abc/status", "payload": {"cumulative": 6254}}
→ 200 {"ok": true}
```

### Subscribe (WebSocket)
```
WS /subscribe?channels=tbg/+/status,brain/embed
◄ {"channel": "tbg/ses_abc/status", "payload": {...}}
```

## Channels Convention

`{plugin}/{sessionID?}/{topic}`

Examples: `tbg/ses_abc/status`, `brain/embed`, `deepseek/balance`

## TypeScript Client

Use `@four-bytes/opencode-plugin-lib` for TypeScript integration:
```typescript
// Server-side
import { BusClient } from "@four-bytes/opencode-plugin-lib";
const bus = await BusClient.connect();
bus.publish("tbg/status", { cumulative: 1234 });

// TUI-side
import { BusTui } from "@four-bytes/opencode-plugin-lib/tui";
const bus = await BusTui.connect();
bus.subscribe("tbg/+/status", (msg) => updateUI(msg.payload));
```

## License

Apache-2.0

# ROADMAP — four-local-bus

> **Status:** Active development. Current binary name `four-opencode-bus`, current Go module
> `github.com/four-bytes/four-opencode-plugin-bus`. This roadmap drives the rename and all
> subsequent feature work.

## Vision

A lightweight, zero-config local pub/sub bus for multi-process terminal applications.
Framework-agnostic: no opencode concepts in the binary or core TS lib.
Session-aware by design: the channel convention and the client API make session scoping
first-class, so every plugin author gets it for free without boilerplate.

```
Publisher (plugin/CLI) ──HTTP POST──────────────────┐
                                                     ├── four-local-bus (Go) ──WS push──► Subscriber (TUI)
Subscriber (TUI)       ──WebSocket long-lived conn───┘
```

## Target Channel Convention

```
{service}/{topic}                  — broadcast, no session
{service}/{sessionId}/{topic}      — session-scoped

Examples:
  brain/status                     broadcast from brain during startup ingest
  brain/ses_abc123/status          scoped to one opencode session
  tbg/ses_abc123/usage             token budget guard, one session
  deepseek/balance                 no session (global balance, one per process)

Wildcard patterns (subscriber side):
  brain/+/status                   all sessions, one topic
  brain/ses_abc123/+               all topics for one session
  brain/+/+                        everything from the brain service
```

## Target API (TypeScript — @four-bytes/local-bus)

```typescript
// ── Publisher (plugin/server side) ─────────────────────────────────────
const bus = await BusPublisher.create("brain");     // service name = namespace

await bus.publish("status", payload);               // → brain/status  (broadcast)

const sess = bus.forSession(toolCtx.sessionID);
await sess.publish("status", payload);              // → brain/ses_123/status
await sess.publish("progress", payload);            // → brain/ses_123/progress

// ── Subscriber (TUI side) ───────────────────────────────────────────────
const sub = await BusSubscriber.create();

const brain = sub.forService("brain");

brain.subscribe("status", cb);                      // → brain/+/status  (all sessions)
brain.broadcast("status", cb);                      // → brain/status    (broadcast only)
brain.forSession(sessionId).subscribe("status", cb);// → brain/ses_123/status
brain.forSession(sessionId).subscribe("+", cb);     // → brain/ses_123/+
```

## Package Layout (Target)

```
four-local-bus/                         ← THIS REPO (renamed)
  cmd/bus/main.go                       Go binary entry point
  internal/
    server/server.go                    HTTP+WS server
    router/router.go                    Channel pub/sub + wildcard routing
    channel/channel.go                  Last-value cache
    discovery/discovery.go              transport.json writer/reader
    transport/                          (Wave 3) Unix socket + TCP abstraction
    encoding/                           (Wave 4) msgpack + JSON codec

@four-bytes/local-bus                   ← NEW TS PACKAGE (extracted from opencode-plugin-lib)
  src/
    publisher.ts                        BusPublisher + SessionPublisher
    subscriber.ts                       BusSubscriber + ServiceSubscriber + SessionSubscriber
    discovery.ts                        Reads transport.json, finds socket/port
    memory-bus.ts                       In-process fallback (no binary)
    types.ts                            BusEnvelope, TransportInfo, etc.
    index.ts

@four-bytes/opencode-plugin-lib         ← EXISTING REPO (becomes thin wrapper)
  Re-exports @four-bytes/local-bus
  Wires discovery to ~/.cache/opencode/... path
  Plugin lifecycle helpers (toast, logger, cache — unchanged)
```

---

## Wave 1 — Rename & Repo Split
> **Goal:** Pure structural refactor. Zero behavior change. All existing tests pass.
> No new features. This is the foundation every subsequent wave builds on.

### Task 1.1 — Rename Go module and binary

**Scope:** `four-opencode-plugin-bus` repo only.

**Changes:**
- Rename Go module in `go.mod`: `github.com/four-bytes/four-opencode-plugin-bus`
  → `github.com/four-bytes/four-local-bus`
- Rename the built binary: `four-opencode-bus` → `four-local-bus`
- Update all internal `import` paths to use the new module path
- Update `AGENTS.md`, `README.md` with new names
- Update `cmd/bus/main.go` if it references the old name anywhere

**Files to touch:**
```
go.mod
cmd/bus/main.go
internal/server/server.go
internal/router/router.go
internal/channel/channel.go
internal/discovery/discovery.go
AGENTS.md
README.md
```

**Do NOT change:**
- The HTTP API surface (`POST /publish`, `GET /subscribe`, `GET /health`)
- The `port.json` discovery path (that changes in Wave 3)
- Any business logic

**Acceptance criteria:**
- `go build -o four-local-bus ./cmd/bus/` succeeds
- `go test ./...` passes
- Old binary name `four-opencode-bus` no longer produced by default build

---

### Task 1.2 — Extract @four-bytes/local-bus TS package

**Scope:** New package, extracted from `@four-bytes/opencode-plugin-lib`.

**Create new repo/package** `@four-bytes/local-bus` with these source files copied
(then simplified to remove opencode-specific bits):

| Source (opencode-plugin-lib) | Destination (local-bus) | Notes |
|---|---|---|
| `src/bus-client.ts` | `src/publisher.ts` | rename class, keep logic |
| `src/bus-tui.ts` | `src/subscriber.ts` | rename class, keep logic |
| `src/discovery.ts` | `src/discovery.ts` | remove opencode path hardcoding |
| `src/memory-bus.ts` | `src/memory-bus.ts` | copy as-is |
| `src/event-bus.ts` | `src/event-bus.ts` | copy as-is |
| `src/types.ts` | `src/types.ts` | copy, extend for Wave 2 |

**Key changes during extraction:**
- `discovery.ts`: remove `~/.cache/opencode/` path. Accept the path as a parameter
  or via `FOUR_LOCAL_BUS_TRANSPORT_FILE` env var. Export a `DiscoveryOptions` interface.
- `publisher.ts` (was `bus-client.ts`): rename `BusClient` → `BusPublisher`.
  Keep the same connect/publish/health logic. Update imports.
- `subscriber.ts` (was `bus-tui.ts`): rename `BusTui` → `BusSubscriber`.
  Keep same WebSocket connect/subscribe/reconnect logic. Update imports.
- Remove all `@opencode-ai/plugin` imports — this package must have zero opencode deps.

**Package.json for @four-bytes/local-bus:**
```json
{
  "name": "@four-bytes/local-bus",
  "version": "0.1.0",
  "type": "module",
  "exports": {
    ".": { "import": "./dist/index.js", "types": "./dist/index.d.ts" },
    "./subscriber": { "import": "./dist/subscriber.js", "types": "./dist/subscriber.d.ts" }
  },
  "devDependencies": { "bun-types": "^1.3.0", "typescript": "^6.0.0" }
}
```

**No peer deps** — this package has no framework dependencies.

**Acceptance criteria:**
- Package builds (`bun run build`)
- `BusPublisher.connect()` and `BusSubscriber.connect()` work as before (same behavior as
  old `BusClient.connect()` and `BusTui.connect()`)
- No imports from `@opencode-ai/*` anywhere in the package

---

### Task 1.3 — Update @four-bytes/opencode-plugin-lib to re-export local-bus

**Scope:** `four-opencode-plugin-lib` repo.

**Changes:**
- Add `@four-bytes/local-bus` as a dependency
- Remove the bus-related source files that were extracted:
  `bus-client.ts`, `bus-tui.ts`, `discovery.ts`, `memory-bus.ts`, `event-bus.ts`
  (but keep the types re-exported for backward compat)
- Add a wrapper `discovery.ts` that calls the local-bus discovery with the
  opencode-specific path (`~/.cache/opencode/plugin-bus/port.json`)
- Re-export everything from `@four-bytes/local-bus` so existing plugin code
  (`import { BusClient } from "@four-bytes/opencode-plugin-lib"`) still works
  via named re-exports (backward compat aliases: `BusClient = BusPublisher`, `BusTui = BusSubscriber`)
- Keep all non-bus code untouched: `toast.ts`, `logger.ts`, `cache.ts`, `tui.ts`,
  `tui-components/`

**Acceptance criteria:**
- All existing plugins that import from `@four-bytes/opencode-plugin-lib` still compile
  without changes
- `BusClient` and `BusTui` are still importable under those names (aliases)

---

### Task 1.4 — Update all plugins to new binary name

**Scope:** Every plugin that references `four-opencode-bus` in auto-start code.

`BusPublisher.connect()` (was `BusClient.connect()`) auto-starts the binary via
`~/.local/bin/four-opencode-bus`. This path must change to `~/.local/bin/four-local-bus`.

**Files to update in `@four-bytes/local-bus/src/publisher.ts`:**
```typescript
// old
join(homedir(), ".local", "bin", "four-opencode-bus")
// new
join(homedir(), ".local", "bin", "four-local-bus")
```

No plugin source files need touching if the re-export aliases are correct.

**Acceptance criteria:**
- Brain plugin connects to bus successfully after renaming
- No reference to `four-opencode-bus` remains in any TS source

---

## Wave 2 — Session-Aware API
> **Goal:** Make `forSession(id)` and `forService(name)` first-class in the TS lib.
> Plugins stop managing channel strings manually. The ALS hack in four-opencode-brain
> is removed and replaced with `bus.forSession(id)`.

### Task 2.1 — BusPublisher session-aware API

**Scope:** `@four-bytes/local-bus/src/publisher.ts`

**New API to implement:**

```typescript
export class BusPublisher {
  // Factory — service name becomes the channel prefix
  static async create(service: string, opts?: BusPublisherOptions): Promise<BusPublisher>

  // Broadcast: publishes to {service}/{topic}
  async publish(topic: string, payload: unknown): Promise<void>

  // Returns a scoped publisher: all publishes → {service}/{sessionId}/{topic}
  forSession(sessionId: string): SessionPublisher

  // Raw channel publish (escape hatch — use sparingly)
  async publishRaw(channel: string, payload: unknown): Promise<void>

  async healthCheck(): Promise<boolean>
  get activePort(): number        // 0 = memory fallback
  get serviceName(): string
}

export class SessionPublisher {
  // Publishes to {service}/{sessionId}/{topic}
  async publish(topic: string, payload: unknown): Promise<void>

  get sessionId(): string
  get service(): BusPublisher
}
```

**Channel derivation (internal):**
```typescript
// Inside BusPublisher:
private channel(topic: string): string {
  return `${this.serviceName}/${topic}`;  // "brain/status"
}

// Inside SessionPublisher:
private channel(topic: string): string {
  return `${this.service.serviceName}/${this.sessionId}/${topic}`;  // "brain/ses_123/status"
}
```

**Keep existing `BusClient` as a deprecated alias:**
```typescript
/** @deprecated Use BusPublisher.create() instead */
export class BusClient extends BusPublisher { ... }
```

The underlying HTTP publish logic is identical — only channel string construction changes.

**Acceptance criteria:**
- `BusPublisher.create('brain')` works
- `.publish('status', data)` sends to channel `brain/status`
- `.forSession('ses_123').publish('status', data)` sends to `brain/ses_123/status`
- All existing tests pass
- New unit tests cover channel derivation for both broadcast and session cases

---

### Task 2.2 — BusSubscriber session-aware API

**Scope:** `@four-bytes/local-bus/src/subscriber.ts`

**New API to implement:**

```typescript
export class BusSubscriber {
  static async create(opts?: BusSubscriberOptions): Promise<BusSubscriber>

  // Returns a service-scoped subscriber
  forService(name: string): ServiceSubscriber

  // Raw pattern subscribe (escape hatch)
  subscribe(pattern: string, cb: BusCallback): Unsubscribe

  close(): void
}

export class ServiceSubscriber {
  // Subscribes to {service}/+/{topic} — all sessions
  subscribe(topic: string, cb: BusCallback): Unsubscribe

  // Subscribes to {service}/{topic} — broadcast channel only (no session segment)
  broadcast(topic: string, cb: BusCallback): Unsubscribe

  // Subscribes to {service}/+/{topic} AND {service}/{topic} (both)
  subscribeAll(topic: string, cb: BusCallback): Unsubscribe

  // Returns a session-scoped subscriber
  forSession(sessionId: string): SessionSubscriber
}

export class SessionSubscriber {
  // Subscribes to {service}/{sessionId}/{topic}
  subscribe(topic: string, cb: BusCallback): Unsubscribe

  // Subscribes to {service}/{sessionId}/+ — all topics for this session
  subscribeAll(cb: BusCallback): Unsubscribe
}
```

**Pattern derivation (internal):**
```typescript
// ServiceSubscriber.subscribe('status', cb) → pattern = "brain/+/status"
// ServiceSubscriber.broadcast('status', cb) → pattern = "brain/status"
// ServiceSubscriber.subscribeAll('status', cb) → TWO subscriptions:
//   "brain/+/status" AND "brain/status"
// SessionSubscriber.subscribe('status', cb) → pattern = "brain/ses_123/status"
// SessionSubscriber.subscribeAll(cb) → pattern = "brain/ses_123/+"
```

The underlying WebSocket subscription logic is identical — only patterns change.

**Keep existing `BusTui` as a deprecated alias:**
```typescript
/** @deprecated Use BusSubscriber.create() instead */
export class BusTui extends BusSubscriber { ... }
```

**Acceptance criteria:**
- `ServiceSubscriber.subscribe()` sends `brain/+/status` to the WebSocket
- `SessionSubscriber.subscribe()` sends `brain/ses_123/status` to the WebSocket
- `subscribeAll` correctly registers multiple patterns
- Existing `BusTui` users still work via alias

---

### Task 2.3 — Remove ALS from four-opencode-brain, use forSession API

**Scope:** `four-opencode-brain` repo, `src/status.ts` and `src/four-opencode-brain.ts`.

**Current state:**
`status.ts` uses `AsyncLocalStorage` (`withSessionId()`) to scope publishes per-session.
`four-opencode-brain.ts` wraps every tool execute in `withSessionId(toolCtx.sessionID, ...)`.

**Target state:**
Use `BusPublisher.create('brain').forSession(sessionId)` instead. Each tool execute creates
a `SessionPublisher` at the top and uses it for all status calls.

**Changes to `src/status.ts`:**
- Remove `AsyncLocalStorage`, `withSessionId`, `_sessionAls`
- Add `export function createSessionStatus(sessionId: string)` that returns an object
  with `updateStatus`, wrapping a `SessionPublisher` internally
- OR: expose `updateStatus(state, opts, publisher?: SessionPublisher | BusPublisher)` —
  see below for preferred design

**Preferred design — session-scoped status module:**
```typescript
// status.ts exports:
export function initBus(service: string): Promise<void>  // call once at startup
export function updateStatus(state, opts?): void          // uses broadcast publisher (startup/ingest)
export function createSessionStatus(sessionId: string) {  // call at start of each tool
  return {
    updateStatus(state: StatusState, opts?: StatusOpts): void
    // uses SessionPublisher internally
  }
}
```

**Changes to `src/four-opencode-brain.ts`:**
- Remove all `withSessionId(toolCtx.sessionID, ...)` wrappers
- At top of each tool `execute`:
  ```typescript
  execute: async (args, toolCtx) => {
    const status = createSessionStatus(toolCtx.sessionID);
    // ... use status.updateStatus() instead of global updateStatus()
  }
  ```
- Auto-ingest still uses global `updateStatus()` (broadcast)

**Acceptance criteria:**
- No `AsyncLocalStorage` import anywhere in brain
- Session A's ingest does not appear in Session B's TUI status bar (manual test)
- Build passes

---

### Task 2.4 — Update other plugins to new API (token-budget-guard, deepseek-meter, etc.)

**Scope:** Any plugin that imports `BusClient` or `BusTui` directly.

For each affected plugin:
- Replace `BusClient.connect()` with `BusPublisher.create('{plugin-name}')`
- Replace `bus.publish('channel/string', data)` with `bus.publish('topic', data)` or
  `bus.forSession(id).publish('topic', data)`
- Replace `BusTui.connect()` with `BusSubscriber.create()`
- Replace raw pattern strings with `forService(name).subscribe(topic, cb)`

This task is plugin-by-plugin. Handle one plugin per sub-task.

---

## Wave 3 — Unix Socket Transport
> **Goal:** Use Unix domain sockets for all local IPC on Linux/Mac.
> Zero config — binary negotiates automatically, writes socket path to discovery file.
> Clients prefer socket over TCP when available.

### Task 3.1 — Go binary: bind Unix socket + TCP simultaneously

**Scope:** `four-local-bus` Go binary, `cmd/bus/main.go` and new `internal/transport/`.

**Behavior:**
1. On startup, attempt to bind a Unix socket at:
   `$XDG_RUNTIME_DIR/four-local-bus-{hash}.sock` (Linux)
   `/tmp/four-local-bus-{hash}.sock` (fallback / macOS)
   Hash = first 8 chars of SHA256 of the process's working directory or PID.
2. Always also bind a TCP listener on a random port (as today).
3. Both listeners serve the identical HTTP+WebSocket handler (`srv.Handler()`).
4. Write `transport.json` with both paths:
   ```json
   { "unix": "/tmp/four-local-bus-abc12345.sock", "port": 48291 }
   ```
5. On shutdown, remove the socket file (add to cleanup).

**New package:** `internal/transport/transport.go`
```go
type Transport struct {
    TCPPort    int
    UnixSocket string  // empty on Windows
}

func Listen() (Transport, []net.Listener, error)
```

**On Windows:** `UnixSocket` is `""`, only TCP is used. The binary must compile and work
on Windows without Unix socket code (use build tags: `//go:build !windows`).

**Discovery file change:** `transport.json` replaces `port.json`.
Write BOTH `port.json` (for backward compat with existing clients) AND `transport.json`.
Drop `port.json` in Wave 4 after all clients are updated.

**Acceptance criteria:**
- `four-local-bus` creates a `.sock` file on startup (Linux/Mac)
- Same file is cleaned up on SIGTERM/SIGINT
- HTTP POST to the socket path (via `curl --unix-socket`) reaches the bus
- WebSocket connection over the socket path works
- `transport.json` contains both `unix` and `port` fields
- `port.json` still written for backward compat
- `go test ./...` passes including new transport tests
- Windows: binary compiles, no Unix socket code runs, TCP only

---

### Task 3.2 — TS lib: prefer Unix socket over TCP

**Scope:** `@four-bytes/local-bus/src/discovery.ts` and `publisher.ts`.

**Discovery changes:**
- Read `transport.json` first, fall back to `port.json`
- Export `TransportInfo`:
  ```typescript
  interface TransportInfo {
    unix?: string;   // socket path (undefined on Windows or if not available)
    port: number;
  }
  ```

**Publisher (HTTP publish):**
Bun supports HTTP over Unix sockets via `fetch` options:
```typescript
const res = await fetch("http://localhost/publish", {
  unix: transport.unix,   // Bun-specific: routes the HTTP request over the socket
  method: "POST",
  ...
});
```
If `transport.unix` is available and `process.versions.bun` is set, use socket.
Otherwise use `http://127.0.0.1:{port}/publish` (Node.js / no socket).

**Subscriber (WebSocket):**
Standard WebSocket API (`new WebSocket(url)`) does not support Unix sockets.
For the subscribe connection, keep TCP (`ws://127.0.0.1:{port}/subscribe`) regardless
of socket availability. The publish path benefits most from Unix socket (high frequency).

**Acceptance criteria:**
- Publish calls use Unix socket on Linux/Mac when running under Bun
- Subscribe connection uses TCP (unchanged)
- Fallback to TCP publish works when socket is unavailable (e.g., Windows, Node.js)
- `discoverTransport()` returns `TransportInfo` with both fields when available

---

## Wave 4 — MessagePack Encoding
> **Goal:** Binary wire format for payload. Smaller messages, faster encode/decode.
> Channel names stay UTF-8 strings (human-readable in logs/debug).
> Negotiated via content-type, not hardwired.

### Task 4.1 — Go binary: add msgpack publish endpoint

**Scope:** `four-local-bus` Go binary, `internal/server/server.go` and new
`internal/encoding/`.

**Dependency:** Add `github.com/vmihailenco/msgpack/v5` to `go.mod`.

**Changes:**
- `POST /publish` checks `Content-Type` header:
  - `application/json` → existing path (unchanged)
  - `application/msgpack` → decode body as msgpack, extract `channel` (string)
    and `payload` (raw bytes, stored as `[]byte`), route to `router.Publish()`
- Router stores payload as `interface{}` — internally it may be `[]byte` (msgpack) or
  `map[string]interface{}` (JSON). The router does NOT re-encode; it passes through.
- WebSocket delivery: if the subscriber negotiated msgpack (Task 4.2), send as msgpack;
  otherwise re-encode payload to JSON. The connection metadata tracks encoding.

**New internal package:** `internal/encoding/encoding.go`
```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
    ContentType() string
}

var JSON    Codec = &jsonCodec{}
var MsgPack Codec = &msgpackCodec{}
```

**Acceptance criteria:**
- `POST /publish` with `Content-Type: application/msgpack` and msgpack body works
- `POST /publish` with `Content-Type: application/json` still works (unchanged)
- `go test ./...` passes including encoding unit tests

---

### Task 4.2 — Go binary: WebSocket subprotocol negotiation

**Scope:** `internal/server/server.go`

**WebSocket subprotocol for encoding negotiation:**
- Client sends `Sec-WebSocket-Protocol: four-local-bus-msgpack` or
  `four-local-bus-json` in the upgrade request
- Server accepts and echoes the subprotocol, records the codec for that connection
- Messages delivered to that connection are encoded with the negotiated codec
- Default (no subprotocol): JSON

**Connection metadata storage:** Add a map in `Server`:
```go
connCodecs map[*websocket.Conn]encoding.Codec
```
Protected by the existing router mutex.

**Acceptance criteria:**
- WebSocket connection with `four-local-bus-msgpack` subprotocol receives msgpack frames
- WebSocket connection with no subprotocol receives JSON frames (unchanged)
- Mixed subscribers on the same channel get correct encoding per connection

---

### Task 4.3 — TS lib: msgpack encode/decode

**Scope:** `@four-bytes/local-bus`

**Dependency:** Add `@msgpack/msgpack` to package.json dependencies.

**Publisher changes (`publisher.ts`):**
```typescript
// When connecting, try msgpack first
const useMsgpack = transport.unix !== undefined || opts?.encoding === "msgpack";

// Publish payload:
if (useMsgpack) {
  body = encode(envelope);            // @msgpack/msgpack encode()
  headers["Content-Type"] = "application/msgpack";
} else {
  body = JSON.stringify(envelope);
  headers["Content-Type"] = "application/json";
}
```

**Subscriber changes (`subscriber.ts`):**
```typescript
// During WebSocket open, request msgpack subprotocol:
const ws = new WebSocket(url, ["four-local-bus-msgpack"]);

// In onmessage, decode based on actual negotiated subprotocol:
ws.onmessage = (ev) => {
  const msg = ws.protocol === "four-local-bus-msgpack"
    ? decode(ev.data as ArrayBuffer)   // @msgpack/msgpack decode()
    : JSON.parse(ev.data as string);
  // ... route to subscribers
};
```

**Acceptance criteria:**
- Messages round-trip correctly with msgpack encoding
- Fallback to JSON when server does not support msgpack subprotocol
- `FOUR_LOCAL_BUS_DEBUG=1` env var forces JSON encoding (for curl/websocat debugging)

---

## Wave 5 — Hardening & Observability
> **Goal:** Production-ready reliability. Metrics, structured logging, config file.

### Task 5.1 — Structured logging in Go binary

Replace `log.Printf` calls with structured JSON logs to stderr:
```json
{"ts":1234567890,"level":"info","msg":"subscriber connected","addr":"unix:/tmp/..."}
{"ts":1234567890,"level":"info","msg":"message routed","channel":"brain/ses_abc/status","subscribers":2}
```

Use stdlib `log/slog` (Go 1.22+). No external logging deps.

Add `--log-level` flag (default: `warn` to stay quiet; `debug` for dev).

---

### Task 5.2 — GET /metrics endpoint (Prometheus text format)

```
# HELP bus_messages_total Total messages published
# TYPE bus_messages_total counter
bus_messages_total{channel="brain"} 1234

# HELP bus_subscribers_active Active subscriber connections
# TYPE bus_subscribers_active gauge
bus_subscribers_active 3
```

Tracked per-service (first segment of channel name). No per-session metrics
(too many cardinality labels).

---

### Task 5.3 — Config file support

`~/.config/four-local-bus/config.json` (XDG config dir):
```json
{
  "socket": "/tmp/my-bus.sock",
  "port": 9999,
  "idleTimeoutSec": 60,
  "logLevel": "warn"
}
```

All fields optional. CLI flags take precedence. Env vars (`FOUR_LOCAL_BUS_PORT`,
`FOUR_LOCAL_BUS_SOCKET`) take precedence over config file.

---

### Task 5.4 — # multi-segment wildcard

Currently `+` matches exactly one path segment. Add `#` as a trailing wildcard
matching zero or more remaining segments:

```
brain/#          matches brain/status, brain/ses_123/status, brain/ses_123/progress
brain/ses_123/#  matches brain/ses_123/status, brain/ses_123/progress
```

Rules:
- `#` must be the last segment (or only segment after a `/`)
- `brain/#/status` is invalid (# must be terminal)
- Validate on subscribe; return error for invalid patterns

Implement in `router.go` `matchPattern()` function. Add tests.

---

## Implementation Notes for AI Dev Sidecars

### Reading this roadmap
Each Wave is independent of the next. Within a Wave, Tasks may have dependencies
(noted inline). Complete Wave 1 fully before starting Wave 2. Waves 3 and 4 are
parallel-capable once Wave 2 is done.

### Repo locations (current)
```
Go binary:    ~/four-opencode-plugin-bus/    (to be renamed to ~/four-local-bus/)
TS lib:       ~/four-opencode-plugin-lib/    (to be split)
Brain plugin: ~/four-opencode-brain/
```

### Go conventions (bus binary)
- No external dependencies beyond stdlib and `gorilla/websocket` (Wave 4 adds msgpack)
- All new packages go under `internal/`
- Table-driven tests in `*_test.go` alongside the package
- CGO_ENABLED=0 always — static binary, no libc dependencies
- Build tag `//go:build !windows` for Unix socket code

### TS conventions (local-bus lib)
- Bun runtime, ESM modules, TypeScript strict mode
- No `any` except at framework boundaries
- No opencode imports (`@opencode-ai/*`) — this is a generic lib
- Build: `bun run build` → `dist/`
- All exported classes have JSDoc

### Test requirements per task
Every task must include:
1. **Unit tests** for the new logic (channel derivation, pattern matching, codec)
2. **Integration smoke test** where appropriate (start binary, connect client, publish, verify receipt)
3. **Backward-compat test**: old API still works after new API is added

### What to avoid
- Do not add opencode-specific concepts to `@four-bytes/local-bus` or the Go binary
- Do not break the HTTP API surface — external tooling may use it
- Do not change channel routing logic while adding transport layers (keep concerns separate)
- Do not make msgpack mandatory — JSON fallback must always work

### Definition of done (per task)
- [ ] Code compiles with no errors or warnings
- [ ] All existing tests pass
- [ ] New tests written and passing
- [ ] AGENTS.md updated if architecture changes
- [ ] HISTORY.md entry added

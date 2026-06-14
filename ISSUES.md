**Status:** Last reviewed 2026-06-14. 2/2 fixed (brain), 3/3 fixed (plugin-lib), 2/2 fixed (local-bus), 2/2 fixed (context-curator).

# Known Issues

## #1 — Orphan processes: no startup idle timer

**Symptom:** Multiple `four-local-bus` processes accumulate over time. Each new opencode
start may spawn a new bus binary while old ones linger.

**Root cause:** `startIdleTimer()` is only called when a subscriber *disconnects* and
`SubscriberCount() == 0`. If no subscriber ever connects (e.g., a bus process was spawned
in a race but the TUI discovered a different instance), the idle countdown never starts and
the orphan runs indefinitely.

**Location:** `internal/server/server.go` — `New()` constructor.

**Fix:** Add a startup idle timer in `cmd/bus/main.go` that sleeps for a longer grace period
(5 min) and then checks `HasEverHadSubscriber()`. If no subscriber has ever connected, the
bus is an orphan from a lost race (see Issue #2) and shuts itself down via SIGTERM.
The point-in-time `HasSubscribers()` would be racy (subscribers can briefly be zero between
a disconnect and the 30 s post-disconnect idle timer firing), hence the dedicated
`HasEverHadSubscriber()` flag in `internal/server/server.go`.

```go
// In cmd/bus/main.go, after starting the server:
go func() {
    time.Sleep(5 * time.Minute)
    if !srv.HasEverHadSubscriber() {
        fmt.Println("four-local-bus: no subscribers after 5m, shutting down")
        sigCh <- syscall.SIGTERM
    }
}()

// In internal/server/server.go:
func (s *Server) HasEverHadSubscriber() bool {
    // returns s.everHadSubscriber under s.subscriberMu
}
```

---

✅ FIXED — commit 6d8656c (startup idle timer raised to 5 minutes to absorb slow TUI startup).

## #2 — Race condition: concurrent spawns produce extra bus processes

**Symptom:** Two callers of `BusClient.connect()` (in `@four-bytes/opencode-plugin-lib`)
can both conclude the bus is absent at the same moment and both call `startBus()`, spawning
two binaries. Both write to `port.json`; one "wins" and one is forever orphaned (Issue #1
above, made worse by the missing startup timer).

**Location:** `@four-bytes/opencode-plugin-lib/src/bus-client.ts` — `connect()` / `startBus()`.

**Fix:** Add a module-level spawn lock (promise deduplication) so concurrent connect calls
share a single spawn attempt. See ISSUES.md in `four-opencode-plugin-lib`.

✅ FIXED — see `four-opencode-plugin-lib` commit b96489a (`_spawnLock` in `BusClient.connect()`).

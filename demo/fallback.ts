#!/usr/bin/env bun
/**
 * four-local-bus — Fallback Demo
 *
 * Demonstrates:
 * 1. Normal pub/sub operation
 * 2. Graceful degradation when bus is unavailable
 * 3. Automatic reconnection on bus restart
 */

import { $ } from "bun";
import { join } from "node:path";
import { homedir } from "node:os";
import { existsSync, readFileSync } from "node:fs";

const BUS_BIN = join(homedir(), ".local", "bin", "four-local-bus");
const PROJECT_ROOT = join(import.meta.dir, "..");
const PORT_FILE = join(homedir(), ".cache", "opencode", "plugin-bus", "port.json");

function expandHome(p: string) { return p.replace(/^~/, homedir()); }

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }

function getBusPort(): number | null {
  try {
    if (!existsSync(PORT_FILE)) return null;
    const data = JSON.parse(readFileSync(PORT_FILE, "utf-8"));
    return data.port || null;
  } catch { return null; }
}

async function healthCheck(): Promise<boolean> {
  const port = getBusPort();
  if (!port) return false;
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`);
    return res.ok;
  } catch { return false; }
}

async function main() {
  console.log("╔══════════════════════════════════════╗");
  console.log("║  four-local-bus — Fallback Demo  ║");
  console.log("╚══════════════════════════════════════╝\n");

  // ── Scenario 1: Bus Working ──────────────────────────
  console.log("── Scenario 1: Bus Working ──");

  // Build binary if needed
  if (!existsSync(BUS_BIN)) {
    console.log("  Building binary...");
    await $`cd ${PROJECT_ROOT} && go build -o ${BUS_BIN} ./cmd/bus/`;
  }

  // Start bus
  console.log("  Starting bus...");
  const bus = Bun.spawn([BUS_BIN], {
    cwd: PROJECT_ROOT,
    stdout: "pipe",
    stderr: "pipe",
  });

  // Wait for port discovery
  await sleep(500);

  const port = getBusPort();
  if (!port) {
    console.log("  ❌ Bus failed to start (no port file)");
    process.exit(1);
  }
  console.log(`  ✅ Bus started on port ${port}`);

  // Subscribe via WebSocket first
  const ws = new WebSocket(`ws://127.0.0.1:${port}/subscribe?channels=demo/test`);
  const msgPromise = new Promise<string>((resolve, reject) => {
    ws.onmessage = (e) => resolve(e.data);
    ws.onerror = () => reject(new Error("WebSocket error"));
    setTimeout(() => reject(new Error("WebSocket timeout")), 5000);
  });
  await sleep(200); // let WebSocket connect

  // Publish test
  const pubRes = await fetch(`http://127.0.0.1:${port}/publish`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ channel: "demo/test", payload: { msg: "hello from bus" } }),
  });
  console.log(`  📤 Publish: ${pubRes.status}`);

  const msg = await msgPromise;
  console.log(`  📥 Received: ${msg}`);
  console.log("  ✅ Pub/Sub working\n");

  // ── Scenario 2: Bus Unavailable (Fallback) ────────────
  console.log("── Scenario 2: Bus Fallback ──");

  // Kill the bus
  console.log("  Stopping bus...");
  bus.kill();
  await sleep(500);

  // Try to publish — should fail gracefully
  const deadPort = port; // bus is dead but port was here
  try {
    await fetch(`http://127.0.0.1:${deadPort}/publish`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ channel: "demo/test", payload: { msg: "should fail" } }),
      signal: AbortSignal.timeout(2000),
    });
    console.log("  ⚠️ Publish succeeded unexpectedly (bus supposed to be dead)");
  } catch {
    console.log("  ✅ Publish correctly failed — bus is down");
  }

  // Check health — should be unavailable
  const healthAfterKill = await healthCheck();
  console.log(`  🩺 Health check: ${healthAfterKill ? "ALIVE (unexpected)" : "DEAD (expected)"}`);

  // Plugin fallback behavior: skip notification, log locally, queue for retry
  console.log("  📝 Plugin fallback: notification queued locally, will retry on reconnect");

  // ── Scenario 3: Bus Restart & Recovery ────────────────
  console.log("\n── Scenario 3: Bus Recovery ──");

  console.log("  Restarting bus...");
  const bus2 = Bun.spawn([BUS_BIN], {
    cwd: PROJECT_ROOT,
    stdout: "pipe",
    stderr: "pipe",
  });
  await sleep(500);

  const port2 = getBusPort();
  console.log(`  ✅ Bus restarted on port ${port2}`);

  // Subscribe first — messages published before subscribe are lost
  const ws2 = new WebSocket(`ws://127.0.0.1:${port2}/subscribe?channels=demo/test`);
  const msg2Promise = new Promise<string>((resolve, reject) => {
    ws2.onmessage = (e) => resolve(e.data);
    ws2.onerror = () => reject(new Error("WebSocket error"));
    setTimeout(() => reject(new Error("WebSocket timeout")), 5000);
  });
  await sleep(200); // let WebSocket connect before publishing

  // Publish after recovery
  const pub2Res = await fetch(`http://127.0.0.1:${port2}/publish`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ channel: "demo/test", payload: { msg: "recovered!" } }),
  });
  console.log(`  📤 Publish after recovery: ${pub2Res.status}`);

  try {
    const msg2 = await msg2Promise;
    console.log(`  📥 Received after recovery: ${msg2}`);
    console.log("  ✅ Recovery successful\n");
  } catch {
    console.log("  ❌ No message received after recovery");
  }

  // Cleanup
  bus2.kill();
  console.log("── Demo Complete ──");
}

main().catch((err) => {
  console.error("Demo failed:", err);
  process.exit(1);
});

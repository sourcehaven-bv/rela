---
id: TKT-ZYM0B6
type: ticket
title: Replace sleep-based sync with deterministic signals in watcher/SSE tests
kind: enhancement
priority: medium
effort: s
status: done
---

## Goal

A test-quality review identified the sleep-based synchronization in the fsnotify
watcher and SSE tests as the repo's main CI flake risk. Replace the fixed sleeps
with deterministic synchronization.

## Scope

### internal/storage/watcher_test.go

- Drop the three 100ms "give the watcher time to start" sleeps: `NewWatcher` registers all fsnotify watches synchronously, so events occurring after it returns are queued and delivered once `Start` drains them.
- `TestWatcher_AutoWatchesNewDirectories`: the event loop auto-watches new directories asynchronously — wait for the watch via a `waitWatched` poll-with-deadline helper on `WatchList()` instead of a fixed 150ms sleep.
- `TestWatcher_IgnoresNonMatchingExtensions`: convert from negative evidence (200ms quiet window) to positive evidence — write the non-matching `.txt`, then a matching `.md` sentinel; the sentinel's arrival proves the `.txt` event was processed and filtered.

### internal/dataentry SSE tests

- `flusherRecorder` signals each `Flush` on a channel with an `awaitFlush(t)` helper. The SSE handler flushes after subscribing + writing the keepalive and after each event, so flushes are exact synchronization points.
- `TestHandleSSEHeaders`, `TestHandleSSEReceivesEvent`, `TestSecuredRouter_AllowsSameOriginSSE`: replace 50ms/20ms sleeps with `awaitFlush`.
- `TestNewRouterSSEEndpoint` was broken (no-op cancel, leaked handler goroutine, asserted nothing) — rewritten to wait for the keepalive flush, cancel, join the goroutine, and assert the `text/event-stream` content type.

## Out of scope

- The 100ms quiet window in `sse_audit_isolation_test.go` — inherent to its assert-nothing-happens confidentiality guard.
- The 200ms bounded stress duration in `TestConcurrentReadDuringOnReload` — a stress budget, not a sync sleep.
- Deliberate server-handler delays in timeout tests (`lua/http_test.go`, `ai/openai_test.go`).

## Verification

- `go test -race -count=10 -run TestWatcher ./internal/storage/` passes.
- `go test -race -count=3` on the dataentry SSE/broker tests passes.
- Full `just ci` green.

PR: https://github.com/sourcehaven-bv/rela/pull/945

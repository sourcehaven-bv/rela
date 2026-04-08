---
id: BUG-FMS1
type: bug
title: rela-server page loads hang for seconds when file watcher fires
description: 'rela-server data-entry web app intermittently hangs on page load for several seconds before any subresources start loading. Root cause appears to be writer-priority starvation on the App.mu RWMutex (held by reloadLockMiddleware on every /api/* request) when the file watcher fires onReload concurrently with a slow read handler. The locking is also architecturally unnecessary — App.Cfg / styleMap / palette are rarely-changing snapshots that should live behind atomic.Pointer, and the cached meta/g aliases duplicate state already protected by Workspace.mu. Fix: replace App.mu with atomic snapshot, drop the lock-upgrade dance, add a regression test and a pprof debug endpoint.'
priority: high
effort: m
why1: Page loads hang for several seconds in Firefox when the user has the SPA open and is editing project files concurrently.
why2: All /api/* requests block waiting for App.mu because a writer (onReload) is queued behind a slow RLock holder, and Go's RWMutex blocks new readers once a writer is waiting.
why3: 'Empirical: a stress harness drove 4 concurrent Chromium users + ~75 file-watcher events in 60s, then 4 concurrent Firefox users + 7 minutes of similar load, against the 998-file tickets project. The server-side schema canary stayed at p99 < 50ms throughout (Chromium: <5ms, Firefox: <14ms). pprof goroutine dumps captured during slow operations show no goroutines blocked on App.mu, no writers queued, total ~25 goroutines. The lock contention hypothesis is not supported by reproduction.'
why4: The architectural critique (request-scoped App.mu RLock + lock-upgrade dance + RWMutex writer priority) is still valid as an anti-pattern, but it does not explain the user's specific symptom. A separate experiment (TKT-ZIEV) tested the next hypothesis — HTTP/1.1 connection-pool exhaustion in Firefox — and that was also disproved (h2c made the SSE error rate unchanged). The user-reported hang has some other cause we have not yet isolated.
why5: 'We were diagnosing from HARs without server-side instrumentation and reached for the most architecturally suspicious thing in the request path (request-scoped RWMutex) without confirming via reproduction first. The systemic fix is exactly what we built: a stress harness + pprof endpoint that lets us *measure* root causes instead of guess. Prevention: never refactor concurrency primitives based on a HAR alone.'
prevention: Stress harness in frontend/stress/ + pprof endpoint on rela-server now exist as the standard tool for diagnosing intermittent server hangs. Future bugs of this shape should be reproduced in the harness before any code changes are proposed. The harness includes a schema canary that bypasses the browser, so server-side latency is always measurable independently of client-side issues.
status: wont-fix
---

## Symptom

Loading the rela-server data-entry web app in Firefox sometimes hangs for "a
long long time" before any subresources load. Eventually it may load, sometimes
it never does. Captured HARs show all subresource and `/api/v1/*` requests in
`status: 0` with empty timings, then later either succeeding in a single fast
burst or remaining failed.

## What I ruled out (with reproduction)

- **Not the security middleware**: replayed the failing requests with curl using the same headers (Referer, Sec-Fetch-Mode: cors, Sec-Fetch-Site: same-origin, large Cookie header) — server returns 200 instantly. Loaded the SPA in headless Chromium against the live server — page renders fine, 25 entity rows shown, schema/config loaded.
- **Not a CORS issue**: same-origin requests don't go through CORS rejection on the server.
- **Not the SSE handler holding a lock forever**: `handleSSE` is registered on the *outer* mux (`router.go:29-30`), not under `reloadLockMiddleware`, so it doesn't hold `App.mu.RLock()`.
- **Not slow git status**: `gitOps` is nil in this project, the handler short-circuits in <1ms.

## Root cause hypothesis (architectural — needs goroutine dump to confirm)

`internal/dataentry/watcher.go:254` defines `reloadLockMiddleware` which holds
`App.mu.RLock()` for the **entire duration** of every `/api/*` request.
`App.mu.Lock()` (write) is taken by:

- `onReload` (the file watcher callback at `watcher.go:152`) on every debounced file change.
- Every mutating handler in `api_v1.go` and `handlers_api.go` via a **lock-upgrade dance** (`api_v1.go:367-374`):

```go
// Need write lock for creation
a.mu.RUnlock()
a.mu.Lock()
defer func() {
    a.mu.Unlock()
    a.mu.RLock()
}()
```

Two failure modes follow from this:

1. **Writer-priority starvation under slow readers.** Go's `sync.RWMutex` blocks new RLock acquisitions as soon as a writer is queued. If any RLock holder is doing I/O (entity list walking the graph, command exec streaming SSE under `inner`, automation scripts, slow filesystem) and the file watcher fires `onReload` during that window, the writer queues. From that moment **every concurrent page-load request blocks** behind the writer. Firefox opens 6+ parallel connections on a fresh page load, so they all stack behind the same event.

2. **Lock-upgrade race.** Between `RUnlock()` and `Lock()` in mutating handlers, another goroutine can win the writer slot. The handler then waits for the writer. If the file watcher debounce fires repeatedly (likely with the user's `tickets/` project of 998 files plus IDE/git activity), writers stack up and readers are locked out for the duration of all of them.

Both scenarios match the "long long time then sometimes loads" symptom: the lock
eventually drains, the queued requests fire in a single sub-50ms burst (which is
what the second HAR shows: every request started within 43ms of each other once
unblocked).

## The locking is not actually needed

`App.mu` protects:

- `App.Cfg` (data-entry config) — rarely changes, only on file edit
- `App.meta` / `App.g` — convenience aliases that mirror `Workspace.Meta()` / `Workspace.Graph()`
- `App.styleMap` / `App.styledTypes` / `App.userPalette` / `App.palette` — derived from Cfg + meta, rebuilt on reload
- `App.openAPIGen` metamodel snapshot — derived from meta

None of these need a global RWMutex with request-scoped locking:

- `Workspace` already has its own `Workspace.mu` and exposes `RLock()` / `RUnlock()` for consumers that need a consistent metamodel + graph snapshot. The dataentry layer's `meta`/`g` aliases are pure caching that can be replaced by direct `a.ws.Meta()` / `a.ws.Graph()` calls.
- `Cfg`, `styleMap`, `palette`, etc. are exactly the case `atomic.Pointer[T]` was designed for: rarely-changing snapshot, hot read path. A handler does one atomic load at entry and reads from that pointer for the whole request — guaranteed consistent, zero contention, writer just does `Store`.
- Mutating handlers don't need to take a global write lock at all — they should call `a.ws.CreateEntity(...)` etc., and the workspace serializes those internally.

## Acceptance criteria

1. `reloadLockMiddleware` is deleted (or reduced to setting `Cache-Control: no-cache` only).
2. `App.Cfg` and derived fields (`styleMap`, `styledTypes`, `userPalette`, `palette`) live behind a single `atomic.Pointer[appSnapshot]`. `onReload` builds a new snapshot and atomically swaps it.
3. `App.meta` / `App.g` aliases removed; handlers call `a.ws.Meta()` / `a.ws.Graph()` directly, or take `a.ws.RLock()` for the small set that need a multi-call consistent view.
4. The `RUnlock(); Lock(); defer Unlock+RLock` dance is deleted from every mutating handler.
5. `OpenAPIGen.UpdateMetamodel` is concurrency-safe internally (its own mutex or atomic pointer).
6. **No new concurrency issues introduced**: existing tests pass (`just test`, `just test-coverage`).
7. **A regression test reproduces the original hang and verifies the fix**: spin up an `App`, hold an SSE/long-lived request, fire the file watcher repeatedly, and assert that `/api/v1/_schema` calls complete in <100ms. Without the fix this should hang; with it, it should pass.
8. **A new pprof debug endpoint** is added behind `--debug-pprof` (loopback-only) so future hangs can be diagnosed from a goroutine dump instead of speculation.

## Why (impact)

The data-entry web app is the primary UI for rela. When it hangs on every other
reload, users blame "rela is slow" and stop using the live-reload feature. The
hang is intermittent, which makes it hard to diagnose without server-side
instrumentation — hence the pprof endpoint requirement.

## 5-Whys

- **why1**: Page loads hang for several seconds in Firefox when the user has the SPA open and is editing project files concurrently.
- **why2**: All `/api/*` requests block waiting for `App.mu` because a writer (`onReload`) is queued behind a slow RLock holder, and Go's RWMutex blocks new readers once a writer is waiting.
- **why3**: `reloadLockMiddleware` was introduced to give every request a consistent snapshot of `App.Cfg`/`meta`/`g`/`styleMap` across the request, and there is no finer-grained mechanism to do that.
- **why4**: The fields it protects are a mix of (a) cached aliases of `Workspace` state that already has its own lock and (b) rarely-changing config snapshots — neither of which actually needs a request-scoped global RWMutex. The middleware is a sledgehammer chosen because it was the simplest thing that worked at write time.
- **why5**: There is no architectural guideline in the project for "how to expose a rarely-changing snapshot to many concurrent readers", so when the requirement appeared the author reached for the most familiar primitive (RWMutex around the whole App struct) instead of `atomic.Pointer`. Same reason the lock-upgrade dance was added later: it was the only thing compatible with the existing middleware, even though it's racy.

## Prevention

- Document the "atomic snapshot for rarely-changing state" pattern in `internal/dataentry/CLAUDE.md` (or wherever the package's design notes live).
- Add a `go vet` / linter rule (or a code review checklist item) flagging any `Lock()` call inside an HTTP handler that's already wrapped by a middleware taking `RLock()`.
- Add the pprof debug endpoint as part of this fix so future intermittent hangs are diagnosable from goroutine dumps in seconds, not from speculative HAR analysis over multiple back-and-forth turns.

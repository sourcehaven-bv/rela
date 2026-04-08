---
id: TKT-9NFK
type: ticket
title: Delete App.mu and reloadLockMiddleware
kind: refactor
priority: medium
effort: s
status: backlog
---

## Problem\n\nAfter TKT-PYN1 and TKT-WYYP land, `App.mu` has no job. Handlers read state via `atomic.Pointer.Load()` (no lock needed). Mutations serialize via `writeMu` (separate lock). Reloads publish via `state.Store()` (no lock needed). The RWMutex is pure overhead.\n\nThis ticket deletes it. It's the simplification high point of the series.\n\nSee `.ignored/locking-alternatives.md` §5 Stage 4.\n\n## Depends On\n\n- TKT-WYYP (writeMu split)\n\n## Scope\n\n**In scope:**\n\n- Delete `App.mu sync.RWMutex` from the `App` struct.\n- Delete `reloadLockMiddleware` from `internal/dataentry/watcher.go`.\n- Remove the middleware from the router chain.\n- Delete/update any tests that manually simulated the middleware via `app.mu.RLock`.\n- Reload path (`onReload`) publishes via `state.Store(newState)` only; no lock acquisition.\n- Verify that reloads now happen truly concurrently with reads (race test).\n\n**Out of scope:**\n\n- Any other lock changes.\n- Any behavior change visible to clients.\n\n## Acceptance Criteria\n\n1. `App.mu` field is deleted.\n2. `reloadLockMiddleware` is deleted.\n3. The router no longer wires `reloadLockMiddleware`.\n4. Handlers that previously called `a.mu.RLock / RUnlock` directly (e.g. `StartGitFetch`) are updated.\n5. `onReload` publishes via atomic store only.\n6. A test specifically verifies that reloads do not block concurrent reads.\n7. All tests pass under `-race`.\n8. No behavior change observable from outside.\n\n## Risk\n\nThis is the stage where a mistake is most visible: any code path that still assumes the middleware guarantees exclusive write access (without `writeMu`) will now race. The writeMu refactor in TKT-WYYP must be complete and correct before this ticket starts.\n\nAlso: `just test` must explicitly pass under `-race`, with a new test for concurrent-reload-during-read.

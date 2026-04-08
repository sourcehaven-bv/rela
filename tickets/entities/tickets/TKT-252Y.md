---
id: TKT-252Y
type: ticket
title: Replace Workspace.mu with atomic.Pointer
kind: refactor
priority: medium
effort: s
status: done
---

## Problem\n\n`Workspace.mu` (sync.RWMutex at `internal/workspace/workspace.go:98`) exists almost exclusively to protect pointer swaps: `searchIdx` and `meta` are replaced atomically during `Sync()` / `Reload()`, and readers take RLock just long enough to capture the pointer. This is the textbook case for `atomic.Pointer[T]`.\n\nSee `.ignored/locking-alternatives.md` §2.2 for the full reasoning. Verdict: unambiguous win, zero trade-offs.\n\n## Scope\n\n**In scope:**\n\n- Replace `searchIdx *search.Index` with `searchIdx atomic.Pointer[search.Index]`\n- Replace `meta *metamodel.Metamodel` with `meta atomic.Pointer[metamodel.Metamodel]`\n- Update `Sync()`, `Reload()`, `Meta()`, `Search()`, `indexEntity()`, `removeFromIndex()`, and any other callers to use `Load()` / `Store()`\n- Keep `Workspace.mu` and its public `RLock()` / `RUnlock()` exports for now IF any external caller still uses them. Otherwise delete them.\n- Audit callers of `workspace.RLock()` / `workspace.RUnlock()` to confirm they're no longer needed.\n\n**Out of scope:**\n\n- Any change to `App.mu` (separate tickets)\n- Any change to `Graph.mu` (staying as-is)\n- Any change to script execution contracts\n- Performance benchmarking (this is a correctness refactor; perf change is expected to be within noise)\n\n## Acceptance Criteria\n\n1. `Workspace.mu` is either deleted or has no meaningful uses left (only protecting fields that are now atomic).\n2. `Workspace.Search()` no longer takes any lock.\n3. `Workspace.Meta()` no longer takes any lock; returns the current metamodel pointer via atomic load.\n4. `Sync()` and `Reload()` publish via `Store()` instead of lock-held assignment.\n5. All existing tests pass under `-race` (`just test`).\n6. No behavior change observable to any caller.\n\n## Why This Ticket First\n\nThis is the smallest, lowest-risk refactor in the series. It establishes the pattern that `AppState` in ticket T2 will follow, and it validates the approach on a smaller surface area before touching the hotter `App.mu` path.

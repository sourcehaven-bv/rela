---
id: IMPL-RESP
type: implementation-checklist
title: 'Implementation: Wire workspace search backend as fsstore Observer (drop Subscribe goroutine)'
status: done
---

## Implementation

- [x] Unit tests written for new code
- [x] Integration tests written
- [x] All edge cases from planning handled
- [x] Code follows project patterns (consumer-side interfaces)
- [x] No silent failures

**Summary of changes:**

- `internal/app/factory.go` — `FSFactory.AddObserver(o)` method (replaces public Observers field per cranky review). Field is now `observers` (private). Plumbed to `fsstore.Config.Observers`.
- `internal/workspace/workspace.go`:
  - New `observerWiringFactory` consumer-side interface declared at workspace (3-method interface satisfied structurally by `*app.FSFactory`).
  - `New`: builds `bleveindex.NewMem()` BEFORE `factory.OpenStore`; if the factory satisfies `observerWiringFactory`, registers the backend via `AddObserver` (no concrete type assertion).
  - `newWorkspace`: takes `searchBackend *bleveindex.Index` as a parameter (was constructed internally).
  - `NewForTest`: builds backend first; default memstore uses `memstore.WithObserver(searchBackend)`.
  - `Close`: store closes first (so no observer calls land after backend close), then search backend.
  - Deleted: `startSearchSubscription`, `searchSubscriptionLoop`, `stopSearch` field, `searchWG` field, `storeEventBufferSize` const.
  - `WithTestStore` godoc rewritten — explicit caveat + remediation.
  - Workspace lifecycle godoc rewritten (observer-based, not Subscribe-based).
- `internal/app/factory_test.go` — new `TestFSFactoryObserversReceiveWrites` covering create + rename (Delete+Put pair) + delete.
- `internal/workspace/bridge_sync_test.go` — new `TestStoreWritesAppearSynchronouslyInSearch`.

**Manual verification:**

- `go build ./...` — clean
- `go test -race ./...` — all packages pass
- `just lint` — 0 issues
- `just arch-lint` — OK
- `just ci` — full pipeline passes

**Acceptance criteria:**

1. ✅ `grep subscriptionLoop\|stopSearch\|searchWG internal/workspace/*.go` returns zero
2. ✅ `FSFactory.AddObserver` works (verified by `TestFSFactoryObserversReceiveWrites`)
3. ✅ `TestFactoryInitialLoad` passes
4. ✅ Incremental sync proven without goroutine join (verified by `TestStoreWritesAppearSynchronouslyInSearch`)
5. ✅ Net LOC negative for code
6. ✅ All checks pass

**Cranky review (round 2) findings + dispositions:**

| # | Severity | Disposition | Resolution |
|---|----------|-------------|------------|
| 1 | critical | addressed | Flipped Close order: store first (drains observer call sites), then backend |
| 2 | critical | deferred | fsstore swallows observer errors. Pre-existing TKT-LCTG behavior; needs its own ticket for telemetry rework |
| 3 | significant | addressed | Factory mutation hidden behind `AddObserver` method; field is now private |
| 4 | significant | addressed | Consumer-side `observerWiringFactory` interface in workspace; no concrete type assertion |
| 5 | significant | addressed | `WithTestStore` godoc rewritten with remediation guidance |
| 6 | significant | addressed | Added one-line comment to backfill: idempotent EntityPut makes the race window safe |
| 7 | minor | addressed | Test renamed `TestStoreWritesAppearSynchronouslyInSearch` |
| 8 | minor | addressed | Factory test now covers create + rename + delete sequence |
| 9 | minor | addressed | `recordingObserver` captures `*entity.Entity`, exposes `putIDs()` helper |
| 10 | nit | wont-fix | `store` import still used by other test in same file |
| 11 | leverage | deferred | Workspace fixture builder for tests — out of scope |
| 12 | leverage | deferred | bridgePaths helper extraction — out of scope |

**Note on deferred #2:** fsstore's `_ = o.EntityPut(e)` predates TKT-LCTG/Q1JT.
Fixing requires deciding whether observers should error-propagate, slog.Warn, or
both. Separable ticket.

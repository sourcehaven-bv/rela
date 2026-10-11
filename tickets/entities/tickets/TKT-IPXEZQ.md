---
id: TKT-IPXEZQ
type: ticket
title: Move the version sweep out of the store into a versioning service
kind: refactor
priority: medium
effort: m
started: "2026-10-11"
completed: "2026-10-11"
status: done
description: Extract the version-reconciliation sweep from pgstore/sqlitestore into a VersionSweep service owned by appbuild; rename pgstore's mu to changeFeedMu.
---

## Description

The version-reconciliation sweep runs as a goroutine owned by the store:
`pgstore.Store.StartVersionSweep` (internal/store/pgstore/sweep.go:106) and
`sqlitestore.Store.StartVersionSweep` (sqlitestore/sweep.go:83), exposed through
the optional capability `store.VersionSweeper` (store.go:1474) that appbuild
type-asserts (versionsweep_shared.go:71).

The sweep needs nothing from the store except its connection handle. The store
keeps it only to start and stop the goroutine (`s.sweep` behind `s.mu`, stopped
in `Close`). TKT-N0IKN9 already moved versioning to `VersionStore` because "a
store just stores"; the sweep stayed behind for lifecycle reasons only
(pgstore.go:66-70).

Move the sweep into a versioning service next to `VersionStore` in each database
backend, owned and stopped by appbuild.

Origin: archplan threads th-0d00a35a and th-6c2cf2b3.

## Scope

In scope:

- `VersionSweep` in pgstore and sqlitestore: built from the pool or handle,
the `ProjectionProvider` and the `SweepConfig`, with `Start` and `Stop`.
- appbuild builds and starts it; `Services.Close` stops it (only the one it
assembled, per the SharedBase invariant).
- Remove `StartVersionSweep` from both stores and remove
`store.VersionSweeper`.
- pgstore: rename `mu` to `changeFeedMu`. It then guards only change-feed state
(subscribers, nextSubID, listener, closed). Fix its comment, which says it
guards only subscribers and nextSubID, and group the guarded fields.

Out of scope:

- Event-driven wake-up of the sweep. The sweep stays a reconciler over
`content_hash IS NULL`; events are lossy and cannot be its source.
- The shared subscriber fan-out type for all four backends (separate decision,
thread th-6c2cf2b3).
- No change to capture semantics, cadence, the advisory lock or the
one-connection tick.

## Acceptance

- `go list -deps` shows no new imports; arch-lint clean.
- `storetest.RunVersionTests` passes on postgres and sqlite.
- Existing sweep tests (sweep_lock_scope_test.go, sweep_drain_test.go, the
BUG-1DWMYO starvation test) pass unchanged in intent.
- A test proves `Services.Close` stops the sweep goroutine (no leak under
`-race`), and closing one tenant's services does not stop another's sweep.

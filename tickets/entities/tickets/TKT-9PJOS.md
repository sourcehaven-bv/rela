---
id: TKT-9PJOS
type: ticket
title: Reload-during-write conformance test in storetest
kind: test
priority: medium
effort: s
status: backlog
---

## Problem

`storetest` already has `TestConcurrentReloadDuringRead`, but ~24 review
findings flag concurrency hazards specifically around *writes* during reload:
rebuilding the search index from a stale snapshot, validation reading outside
`WithTx` racing against reload, two-phase `Load()` calls observing different
snapshots, etc.

RR-EI58 (addressed) explicitly noted "TestConcurrentReloadDuringRead does not
exercise concurrent writers". RR-065O2 (deferred) asked for the conformance test
to also run against `SafeFS`-wrapped `OsFS`.

## Scope

**In scope**

- Add `TestConcurrentReloadDuringWrite` to
`internal/store/storetest/conformance.go`. Goroutine A repeatedly calls
`Reload()`; goroutine B repeatedly creates/updates entities; assert no data race
(under `-race`) and no intermediate broken state.
- Add a writers-and-readers variant.
- Run the harness against `SafeFS`-wrapped `OsFS` as well as the existing
fsstore/memstore implementations (closes RR-065O2).

**Out of scope**

- Migrating callers to a new reload contract — that is several other tickets.

## Acceptance criteria

- New conformance tests pass on `develop` under `-race`.
- They fail when a regression reintroduces a torn-snapshot read or a
goroutine-leak path.
- Documented in `internal/store/doc.go` "Reload contract" section.

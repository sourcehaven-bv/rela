---
id: TKT-415WA7
type: ticket
title: 'appbuild: widen the three *pgstore.Store concrete-type assertions to interfaces'
kind: refactor
priority: medium
effort: m
status: done
---

## Description

`appbuild` discovers pgstore's optional capabilities by asserting the **concrete
type** `*pgstore.Store`, not an interface. A third backend therefore cannot opt
into the version sweep, derived-schema reconciler or user-state no matter what
interfaces it satisfies.

Sites:

- `internal/appbuild/derivedschema_postgres.go:23` — `st.(*pgstore.Store)`
- `internal/appbuild/userstate_postgres.go:27` — `st.(*pgstore.Store)`
- `internal/appbuild/versionsweep_postgres.go:42,93` — `st.(*pgstore.Store)`
(`stateKVFor`, `startVersionSweepIfSupported`)

Related: `StateKV` is not a `store` interface at all — it is a concrete
`pgstore.StateKV` reached via `pgstore.StateStoreFor(st)`, which does the same
concrete assertion internally. `SetUniqueSpecProvider` and `StartVersionSweep`
are likewise concrete methods.

## Why now

Prerequisite for DEC-LFSYNY (SQLite backend). Identified during that ticket's
design review as work that is easy to miss in an estimate, because it looks like
part of the backend but is not.

Independently valuable regardless of SQLite: it is what makes "pluggable store
backends" (FEAT-CO4YP) true for *smart* backends, not just for the store
interface. The optional-capability pattern already works this way everywhere
else — `HistoryReader`, `Formatter`, `TypeWatermark` are all type-asserted as
interfaces at the wiring site.

## Scope

IN: define consumer-side interfaces for the three capabilities and assert
against those instead of the concrete type. Keep the build-tag structure and the
genuinely-nil-interface fallback contract intact (the doc comments stress that a
*typed* nil would defeat the caller's nil check — preserve that).

OUT: implementing any of these for a new backend. This ticket only removes the
structural block.

## Acceptance criteria

1. No `st.(*pgstore.Store)` assertion remains in `internal/appbuild`.
2. `pgstore` still satisfies every widened interface; the postgres build's
behaviour is unchanged (pinned by the existing wiring tests).
3. A non-pgstore store satisfying the interfaces would be wired identically —
demonstrated by a test double, not by adding a real backend.
4. The nil-fallback contract still holds: a store implementing none of them
still gets the FSKV / in-memory user-state fallbacks.
5. `just arch-lint` and the postgres CI job pass.

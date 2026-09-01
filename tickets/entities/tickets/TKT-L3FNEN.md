---
id: TKT-L3FNEN
type: ticket
title: Promote ProjectionProvider, SweepConfig and VersionStore into internal/store so sweep capabilities are backend-neutral
kind: refactor
priority: low
effort: m
status: done
---

## Description

TKT-415WA7 widened appbuild's capability *discovery* from `st.(*pgstore.Store)`
to interfaces. Two of those interfaces still name pgstore types in their
signatures, so in practice only pgstore can satisfy them — a second backend
would have to import pgstore, which is not real decoupling.

```go
type versionSweeper interface {
    StartVersionSweep(provider pgstore.ProjectionProvider, cfg pgstore.SweepConfig)
}

type versionServiceProvider interface {
    VersionStore() *pgstore.VersionStore
}
```

Same issue one level down: `stateKVFor` calls `pgstore.StateStoreFor(st)`, which
performs its own `st.(*Store)` assertion internally and returns a concrete
`*pgstore.StateKV`.

## Why this matters more than "residual coupling"

Code review of TKT-415WA7 sharpened this. Naming pgstore types in the signatures
is not merely aesthetic debt: **the half-migration opened a real nil-contract
gap.**

Asserting `st.(*pgstore.Store)` bounded the reachable implementations to exactly
one, whose `VersionStore()` is unconditionally non-nil. Widening discovery to an
interface removed that bound while the return type stayed
`*pgstore.VersionStore` — so the code still *read* as though the old guarantee
held. It did not. A nil pointer boxed into `store.VersionService` yields a
NON-nil interface, so every downstream nil-check passes and the panic lands at
write time.

That specific hole is fixed in TKT-415WA7 with explicit nil guards plus
`TestCapabilityPresentButHandleNilYieldsUntypedNil`. The lesson for THIS ticket:
when the return types are finally promoted, the guards must stay — they are what
makes the contract independent of any one implementation.

## Also in scope: stateKVFor was left out entirely

`stateKVFor` still discovers via `pgstore.StateStoreFor(st)`, which type-asserts
`*pgstore.Store` internally (`statekv.go:72`). So a second backend would get
version sweeps, user state and derived schema by interface, then **silently fall
back to node-local FSKV for state**. That partial adoption is worse than none:
it degrades quietly at runtime instead of failing loudly at wiring, and
`stateKVFor`'s own comment documents the consequence (an operator's logo upload
lands on one node and every other keeps serving the old one, with no error
anywhere).

Must be closed before a second backend ships, not merely before it is
"complete".

## Why this was split out rather than done at once

TKT-415WA7 deliberately stopped at discovery. Promoting these types changes
pgstore's public API and touches the sweep, version-store and state-KV surfaces,
so it is a different risk profile from a mechanical assertion swap — and the
assertion swap was the part actually blocking DEC-LFSYNY.

Recording it explicitly so "no concrete assertions remain in appbuild" is not
misread as "the capabilities are backend-neutral". They are not, yet.

## Scope

IN:

- Move `ProjectionProvider` and `SweepConfig` into `internal/store` (or a
neutral package), leaving pgstore aliases if that keeps the diff small.
- Give `VersionStore()` a `store.VersionService` return type rather than the
concrete `*pgstore.VersionStore`. Note `store.VersionService` already exists as
the umbrella interface — this may be as simple as changing the signature.
- Decide `StateStoreFor`: either a `state.KV`-returning interface assertion, or
leave it and document why. Its doc comment argues for a package-level function
over a `Store` method for plimsoll reasons; that rationale should be re-checked,
not assumed still-binding.

OUT: implementing any of it for a second backend.

## Constraint carried forward

**Do not add accessor methods to `pgstore.Store`.** It carries
`//plimsoll:max-exported-methods=38` and an explicit note that a third
capability accessor "should not raise these numbers again". Whatever shape this
takes must not grow that method set.

## Acceptance criteria

1. `internal/appbuild`'s capability interfaces name no pgstore types.
2. A backend outside pgstore could satisfy them without importing pgstore —
demonstrated by a test double in a package that does not import pgstore.
3. pgstore's exported method count does not increase (plimsoll unchanged).
4. Postgres conformance + appbuild wiring suites pass unchanged against a real
database.
5. `just arch-lint`, `just lint` (including `--build-tags postgres`) clean.

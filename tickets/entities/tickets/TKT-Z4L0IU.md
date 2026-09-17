---
id: TKT-Z4L0IU
type: ticket
title: 'Make face→world resolution total: a tie is the chain head alone'
kind: refactor
priority: medium
effort: s
status: review
---

## Description

`validateFacePrimacy` (`internal/metamodel/loader.go`) keys a tie on
`{entityType, face, otherwise}`, but the SPA's `worldForFace`
(`frontend/src/stores/schema.ts`) keys on `{entityType, face}` alone. Two worlds
that lead the same face and differ only in `otherwise:` are therefore two
distinct non-ties to the loader and one unresolved tie to the client.

The client's answer in that case is `undefined` — indistinguishable from "no
world heads this face" — and its own doc tells callers to omit the affordance
rather than guess. So a schema that loads cleanly can leave `worldForFace(type,
face)` with no answer, for a shape the loader deliberately permits.

This makes `worldForFace` a **total** function: every (entity type, face) that
any world heads resolves to exactly one world, or the schema fails to load. Drop
`otherwise` from `primacyKey` so sharing a chain head is sufficient to be a tie,
and the operator breaks it with `primary_for:` as they already do for an
identical-`otherwise:` pair.

## Why the current distinction does not hold

The `otherwise`-keyed tie rests on two worlds answering "a different question
about the entities that lack the face". Post-BUG-HC6I2T that difference is
unobservable for a properly faced type. `ResolveWorldPrimes` fires
`FallbackDefaultState` only when `f.haveDefault` — only when a zero-coordinate
row exists — and a type declaring `faces:` stores no such row.

Verified against `store.ResolveWorldPrimes` directly:

- faced-only entity (`draft` face, no bare row), `otherwise: default`,
chain `[published]` → `map[]` (excluded)
- same world, entity that also has a bare row → `{Face:"", Via:2}`
(`ResolutionFallbackDefault`)

So for a faced type `otherwise: default` and `otherwise: exclude` resolve
identically, and keying the tie on a value that changes nothing splits a genuine
ambiguity into two halves that each look unambiguous.

## Scope

In:

- `primacyKey` loses its `otherwise` field; `sortedPrimacyKeys` loses that
sort term; `validateFacePrimacy`'s doc is rewritten.
- `TestFacePrimacy_SameHeadDifferentOtherwiseIsNotATie` inverts — the same
schema must now fail the load naming `primary_for:`.
- `prototypes/worlds/project/schema.yaml` and any in-tree schema that trips
the tightened rule.

Out (follow-up, tracked separately): removing `otherwise:` as a modelling key
altogether. That reaches four store backends, SQL pushdown in two of them, the
`via: fallback-default` wire value and its SPA badge, and
`docs/content-states.md`. It also weakens a store conformance test that uses
`FallbackDefaultState` as its discriminating verdict.

## Consequence

Narrowing only: a schema that loaded before either still loads or fails with a
named `primary_for:` remedy. No runtime resolution changes, no migration, no
stored data affected.

The unblocked consumer is the `@`-mention picker, which needs to name the world
serving the face being edited (issue #1614). `worldForFace` is tested but has no
component consumer yet, so this lands the invariant before the first caller
depends on it.

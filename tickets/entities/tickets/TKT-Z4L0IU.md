---
id: TKT-Z4L0IU
type: ticket
title: 'Make face→world resolution total: a tie is the chain head alone'
kind: refactor
priority: medium
effort: s
status: done
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

## Why the `otherwise:` exemption is wrong

The exemption asks the wrong question of the right key.

`otherwise:` decides what a world does with an entity that has **none** of the
faces it names. The primacy loop reaches this rule only for a face the type
DECLARES and both worlds LEAD. A reader switching to a face is asking about an
entity that **has** it — there the chain head decides and `otherwise:` is never
consulted, so both worlds hand back the same row.

So the parameter the exemption turned on cannot separate the two worlds on the
one question a face-switch asks. Splitting the key by it turned one genuine
ambiguity into two halves that each looked unambiguous.

This argument is about which **row** a reader is served. It does not rest on how
the two worlds treat entities *lacking* the face — those differ, observably and
by design, and this change narrows none of it.

## A corroborating aside, with its precondition

BUG-HC6I2T removed the requirement that a named face carry a zero-coordinate
row. For a type whose rows are **all** faced, `ResolveWorldPrimes` can never
take its `FallbackDefaultState` arm (it is guarded on `f.haveDefault`), so the
two `otherwise:` values coincide there.

That is an aside, not the reason, and its precondition does not always hold. A
type may still hold bare rows — written before `faces:` was declared, or by a
raw-store path (`perfseed`, `datamigration`, a hand-edited fsstore tree).
`prototypes/perf/project` is an in-tree fixture where they do: `perfseed` writes
every policy at the bare coordinate plus an optional `published` face and never
a `draft` row, so the `editorial` world (chain `[draft, published]`, `otherwise:
default`) genuinely takes the fallback arm.

Verified against `store.ResolveWorldPrimes` with that fixture's shape (one bare
policy row, chain `[draft, published]`):

- `otherwise: exclude` → `map[]`
- `otherwise: default` → `{POL-1: {Face:"", Via:2}}` (`ResolutionFallbackDefault`)

The two differ observably. An earlier draft of this ticket claimed the arm was
unreachable for any faced type and used that as the justification; a code review
falsified it against this fixture. The change stands on the per-type/per-face
argument above, which needs no claim about stored rows.

## Scope

In:

- `primacyKey` loses its `otherwise` field; `sortedPrimacyKeys` loses that
sort term; `validateFacePrimacy`'s doc is rewritten.
- `TestFacePrimacy_SameHeadDifferentOtherwiseIsNotATie` inverts — the same
schema must now fail the load naming `primary_for:`.
- `worldForFace`'s comments in the SPA, which documented the old rule as
current.
- `docs/metamodel.md`, whose `primary_for:` example was itself a schema the
tightened rule rejects.

Out (follow-up, tracked separately): removing `otherwise:` as a modelling key
altogether. That reaches four store backends, SQL pushdown in two of them, the
`via: fallback-default` wire value and its SPA badge, and
`docs/content-states.md`. It also weakens a store conformance test that uses
`FallbackDefaultState` as its discriminating verdict, and — per the aside above
— would change behaviour for types holding bare rows.

## Consequence

Narrowing only: a schema that loaded before either still loads or fails with a
named `primary_for:` remedy. No runtime resolution changes, no migration, no
stored data affected.

The unblocked consumer is the `@`-mention picker, which needs to name the world
serving the face being edited (issue #1614). `worldForFace` is tested but has no
component consumer yet, so this lands the invariant before the first caller
depends on it.

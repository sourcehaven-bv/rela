---
id: RR-PTX67Y
type: review-response
title: Face scoping tested in one direction only, and never with a colliding id
finding: The cross-face test stored on the draft face and read via the default, asserting not-found. Target.Key() is entity.FormatStateRef, so the default face is the BARE id and a named face is "id@face" — meaning "TKT-1" is a prefix of "TKT-1@draft" but not the reverse. A prefix-matching bug is therefore asymmetric, and the tested direction is the one it would not break. Neither direction used two comments sharing an id, so a backend matching on `id` alone while ignoring `target_key` could still pass the cross-face case.
severity: significant
resolution: 'Added three cases to runGetScopingTests: the reverse direction (default face unreachable through a named one), one id on two faces, and one id on two targets — the last two asserting each lookup returns ITS OWN body and author, not merely some row. Mutation-tested with `WHERE id = $2` (ignoring the target): all five scoping cases now fail, where before the collision cases did not exist.'
status: addressed
---

## Finding

AC5 says an id present under `id@draft` must not resolve through the default
face. The test stored on `draft`, read via the default, and asserted
`ErrNotFound` — correct, but only half the boundary.

`Target.Key()` is `entity.FormatStateRef`, where the default face serializes to
the **bare id** and a named face to `id@face`. So one direction compares
`"TKT-1"` against a row keyed `"TKT-1@draft"`, and the other compares
`"TKT-1@draft"` against a row keyed `"TKT-1"`. A prefix-matching bug — the class
this file is full of defences against, see `selectFaces` — breaks only the
second. The suite tested the first.

The sharper gap: neither scoping case had **two comments sharing an id**. With
one row in the store, a backend that ignored `target_key` entirely and matched
on `id` alone would still satisfy the cross-face assertion. Since `Get`'s result
decides whether an `*-own` permission covers a mutation, "returns the right row"
rather than "returns a row" is the property with the security weight, and it was
the one not pinned.

## Resolution

Three cases added to `runGetScopingTests`:

- `the default face is not reachable through a named one` — the reverse
direction, with a comment explaining why it is not redundant.
- `one id on two faces returns each face's own comment` — asserts body AND
author per face, because the author is what the permission check reads.
- `one id on two targets returns each target's own comment` — the same property
across entities.

Verified by mutation: replacing the WHERE clause with `id = $2` alone (ignoring
the target) now fails all five scoping cases. Before these additions the
collision cases did not exist to fail.

The reviewer's claim that a `target_key`-ignoring backend would pass the
cross-face subtest turned out not to hold — it fails on the cross-TARGET case,
because each subtest builds a fresh store and that one happens to contain two
threads. The underlying point stood anyway: nothing pinned "the right row"
directly, and now something does.

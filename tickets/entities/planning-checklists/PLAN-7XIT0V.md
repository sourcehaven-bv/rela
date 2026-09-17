---
id: PLAN-7XIT0V
type: planning-checklist
title: Planning
status: done
---

## Understanding

- [x] Problem understood: `validateFacePrimacy` keys a tie on
`{type, face, otherwise}` while the SPA's `worldForFace` keys on `{type, face}`,
so a schema can load clean and still leave the client's face→world lookup with
no answer.
- [x] Root cause traced: the `otherwise:` exemption rests on two worlds
answering a different question about entities LACKING the face. Verified
empirically that for a faced type they no longer do — `ResolveWorldPrimes` takes
its `FallbackDefaultState` arm only when `f.haveDefault`, and since BUG-HC6I2T a
faced type stores no zero-coordinate row.

## Research

- [x] ~~Research document~~ (N/A: the design question was settled in
conversation against the resolver's own behaviour; the empirical probe is
recorded in the ticket description.)

## Approach

- [x] Approach chosen: drop `otherwise` from `primacyKey`, so sharing a chain
head for a type is the tie and `primary_for:` is the single remedy.
- [x] Alternative considered and rejected: teach the client the `otherwise:`
dimension instead. Rejected because it spreads resolution semantics into the SPA
and leaves the ambiguity real — both worlds still serve the same row to a reader
switching to a face they have.
- [x] Scope deliberately bounded: removing `otherwise:` as a modelling key is a
separate, larger change (four store backends, SQL pushdown, a wire value and its
badge, a conformance test that uses `FallbackDefaultState` as its discriminating
verdict). Tracked separately.

## Security

- [x] Change is narrowing only: a schema that loaded before either still loads
or fails with a named remedy. No runtime resolution change, so no read-path or
ACL behaviour moves.
- [x] No new surface: load-time validation only, operator-shell trust boundary.

## Test plan

- [x] Invert `TestFacePrimacy_SameHeadDifferentOtherwiseIsNotATie` to assert
the load error, and mutation-check it against the pre-change key.
- [x] Add a companion asserting `primary_for:` resolves the differing-
`otherwise:` tie, so the remedy is pinned alongside the rule.
- [x] Validate every in-tree schema still loads.

## Risk

- [x] Risk assessed as low: no in-tree schema trips the tightened rule
(verified against all three prototypes). An out-of-tree schema with two worlds
heading one face and differing `otherwise:` now fails the load, with an error
naming the face and the remedy.
- [x] Migration: none needed — no stored data or resolution behaviour changes.

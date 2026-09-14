---
id: AM-relation-picker-pages-its-candidates
type: automated-measure
title: Relation picker resolves an already-linked target that sits beyond the first candidate page
description: An e2e test that seeds a target type past the server's 100-per-page cap, points an entity's existing relation at an off-page target, then adds a second value in the legacy (non-cards) RelationPicker and asserts BOTH edges are readable afterwards. Guards the class "a client-side consumer treats one page of a paged collection as the complete set" at the one call site that still did, and pins the observable contract (both edges persist) rather than the mechanism, so a future rewrite of the type-resolution path stays covered.
kind: test
location: e2e/tests/relation-picker-large-candidate-set.spec.ts
status: active
---

## Why this shape

The test asserts **server state**, not the absence of a toast. The bug's
signature was that no PATCH was sent at all, so "both edges are readable after
the save" is the only assertion that distinguishes a real fix from a suppressed
error message.

It also asserts its own premise: before touching the UI it re-reads page 1 and
fails with an explicit message if the off-page target is somehow on it. Without
that guard, raising the server's per-page cap would turn the test green while
testing nothing.

## Why e2e rather than a unit test

The defect spans three units that are each individually correct:
`loadCandidates` (fetches a page), `buildOutgoingTypes` (maps what it has), and
`reshapeLegacyToModern` (rejects an unresolvable id). A unit test on any one of
them would have to encode the bug's own assumption to fail. The composition is
the defect, so the guard belongs where the composition runs.

## Related

Same failure class as BUG-5OAQUG (kanban board rendered only page 1), which
introduced `listAllEntities`. That fix was applied to the reporting consumer
only; this measure covers the last remaining one.

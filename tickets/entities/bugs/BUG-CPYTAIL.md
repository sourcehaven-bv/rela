---
id: BUG-CPYTAIL
type: bug
title: "copy `relations: replace` deletes the DEFAULT face's edge and leaves the target's intact"
description: applyCopyEdges listed the target face's edges by tail then called DeleteRelation with the tail dropped. Every backend's DeleteRelation is default-tail-only, so the delete silently removed the DEFAULT face's edge on the same triple — a face the copy has no business touching — and reported success, while the edge being replaced survived. Fixed in TKT-C1XUA8 PR-D by adding DeleteRelationState; filed for traceability.
priority: high
status: backlog
why1: applyCopyEdges called DeleteRelation(rel.From, rel.Type, rel.To), dropping rel.FromFace.
why2: The tail is part of a relation's identity, but DeleteRelation's signature cannot express one, so dropping it silently re-addresses a different edge rather than failing.
why3: No per-tail delete primitive existed — store.RelationData.FromFace's godoc deferred it, saying individual delete of a state-tailed edge "has no consumer before the Step-3 copy kernel and is added then". The copy kernel landed without it.
why4: The ErrNotFound branch at copy_apply.go:64 was written expecting a miss, which made the wrong-edge deletion look like a handled case rather than an unhandled one.
why5: A deferred primitive was tracked only in a godoc comment on an unrelated struct field, so the consumer that was supposed to add it had no gate forcing the question at merge time.
prevention: The conformance suite now carries DeleteRelationStateAddressesTheTailNotTheTriple, named for the hazard, so any backend regressing to default-tail addressing fails. More generally — when a godoc defers work to a named future consumer, that consumer needs a ticket, not a comment.
---

## Symptom

`internal/entitymanager/copy_apply.go` (as shipped in TKT-C1XUA8 PR-C) listed
the target face's edges filtered by tail:

```go
for rel, err := range view.ListRelations(ctx, store.RelationQuery{
    From: plan.targetID, Type: e.relType, FromFace: &tail,   // tail-scoped
}) {
    derr := view.DeleteRelation(ctx, rel.From, rel.Type, rel.To)  // tail DROPPED
```

All three backends address the default tail only:

- pgstore `relation.go`: `... AND from_face = ''`
- memstore `memstore.go`: `defaultTailKey(from, relType, to)`
- fsstore `relation.go`: bare `from + "--" + relType + "--" + to`, identical to
  `relKey(from, "", ...)`

## Impact — reproduction transcript

Reproduced against memstore before fixing, using a throwaway test that made
exactly the call `applyCopyEdges` made. Setup: `PAGE-1` (default face) and
`PAGE-1@published`, each holding its own `references -> SPEC-1` edge — two
distinct relations differing only by tail.

```go
// This is exactly what applyCopyEdges does for `relations: replace` into
// PAGE-1@published: it listed the published-tail edge, then called
// DeleteRelation dropping the tail.
if err := s.DeleteRelation(ctx, "PAGE-1", "references", "SPEC-1"); err != nil {
    t.Fatalf("delete: %v", err)
}
```

Output:

```
=== RUN   TestTailBugDemo
    edges surviving the intended delete of the PUBLISHED-tail edge:
      [PAGE-1@published--references--SPEC-1]
--- PASS: TestTailBugDemo (0.00s)
```

Read that carefully — it is the worst of the three possible outcomes:

1. The delete returned **nil**. Not `ErrNotFound`, so the `!errors.Is(derr,
   store.ErrNotFound)` guard at the call site was never even reached.
2. It deleted the **default face's** edge — a face the copy never addressed
   and had no business modifying.
3. The **published-tail edge it was trying to replace survived**.

An earlier code-reading pass had predicted `ErrNotFound` as the likely
(benign) branch. It was wrong, and only running it showed that. Had the
reading stood, the PR description would have carried the benign version and a
reviewer would reasonably have believed it.

The delete **succeeded**, destroyed the **default** face's edge — which the
copy never intended to touch — and left the published-tail edge it meant to
replace alive. Silent, in all three backends, with no error path reached: the
`ErrNotFound` branch was never taken, so the swallow at `copy_apply.go:64`
was not even the mechanism.

`promote-page` with `relations: replace` is the headline use case in design
doc §9.1, and it was actively destructive.

## Fix

TKT-C1XUA8 PR-D adds `store.DeleteRelationState(ctx, from, face, relType, to)`
across all three backends, reimplements `DeleteRelation` as
`DeleteRelationState(…, "", …)` so the two cannot drift, and addresses the
delete by `rel.FromFace`. Pinned by
`storetest/states.go` `DeleteRelationStateAddressesTheTailNotTheTriple` and by
`TestCopy_ReplaceAddressesTheTargetTail`, both mutation-verified to fail
against the old code.

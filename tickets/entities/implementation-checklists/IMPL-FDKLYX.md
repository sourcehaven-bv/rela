---
id: IMPL-FDKLYX
type: implementation-checklist
title: 'Implementation: faced address on the relation, clone and document routes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Each handler parses its own `{id}` segment with `parseEntityRef`, then feeds the
BARE id to the ACL row gate (face-blind by design) and the REF to the store read
(`getEntityRef` / `GetEntityState`).

Parsing alone is not sufficient, and three further changes were needed:

- **Outgoing edges read at the addressed tail.** `outgoingRelations` queried
without a tail filter, returning the union of every face's edges — so with the
address parsed but the query unchanged, a draft would still have served the
published face's links. New `entityReader.outgoingRelationsOnFace`. Incoming
edges stay entity-level: heads are faceless, so an inbound edge points at the
entity and is shared by its faces.
- **PATCH/DELETE address the right tail** via `tailOfExistingEdge`, which
applies two rules in order: the caller's named face wins when the addressed
entity OWNS the edge (a triple can carry one edge per tail, so discovering a
tail would let `@draft` modify the published edge); otherwise the tail is read
off the existing edge, because a bare address names no face and an incoming
edge's tail belongs to the peer (RR-5MLZCR).
- **CREATE derives the tail** by the same rule `applyRelationsModern` uses:
content-scoped outgoing takes the addressed face, incoming tails at the peer,
identity-scoped takes the zero face.

The document routes additionally validate the segment with
`isSafeStateRefSegment` rather than `isSafePathSegment`, since it is an address
that also keys the on-disk render cache. A faced address keys a distinct cache
entry, which is correct — the faces render different content.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Both tests assert on WHICH ROW was reached, never on a status code. That is
forced by the defect's own nature: memstore keys its index on `FormatStateRef`,
so a suffixed id resolves by accident and a 200-assertion passes against the
broken code.

`TestFacedAddress_RelationsSubTreeReachesItsOwnTail` seeds one target per face
and asserts each address serves only its own.

`TestFacedAddress_SingleRelationPatchHitsItsOwnTail` seeds one triple at TWO
tails and PATCHes one of them.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Both tests were verified to FAIL against the unfixed code, not merely to pass
against the fixed code:

```
without the tail filter:
  draft face serves ["FEAT-DRAFT" "FEAT-PUB"], want ["FEAT-DRAFT"]
  published face serves ["FEAT-DRAFT" "FEAT-PUB"], want ["FEAT-PUB"]

without rule 1 of tailOfExistingEdge:
  published edge note = "published original", want "published EDITED"
  draft edge note = "published EDITED", want "draft original"
```

The second is real cross-face corruption: a PATCH addressed to `@published`
wrote the draft edge.

**A correction worth recording.** My first version of the multi-tail test passed
against BOTH implementations. memstore yields a triple's tails in sorted order,
so a discovered tail is always `draft`; addressing `@draft` then agrees with the
bug. Only addressing `@published` — the tail that does not sort first — made the
test discriminate. This is the same accidental-agreement trap that hid the
underlying bug, reappearing one level up in the test.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No debug code left behind

Patterns followed: `parseEntityRef` as every faced-aware route uses it;
`applyRelationsModern`'s tail rule reused rather than re-derived; `bareEntityID`
left alone on attachments, where per-entity keying is correct.

**Security.** Unchanged in shape and strictly better fed: the ACL row gate now
receives the bare id it always expected, where before a faced address handed it
a string matching no row. These routes failed CLOSED (404), so this is a
reachability fix, not a disclosure one — TKT-O7R2A1 already added the face half
of the read gate. One genuine narrowing: the relations read now returns only the
addressed face's edges, where it previously returned every face's.

**A deliberate 400-vs-404 divergence.** On the document routes a malformed
address is a 400, not the uniform 404 the entity routes give. The segment names
a cache file, the id never reaches the store, and the reserved-segment case
(`/_documents/sales/_EXPORT`) must stay a 400 — a regression I introduced and
caught via `TestExportDocument_RouteShapes`.

**A structural note.** I first threaded `entityRef` from the dispatcher, which
is the better design, but it pushed `handleV1DynamicRoutes` over the gocognit
limit (30 → 38). Per-handler parsing keeps the dispatcher byte-identical. The
dispatcher-level version is what the `entityRef`-as-currency direction in
`faced-route-address-parse-test` would eventually want, and it needs the
dispatcher decomposed first.

Gates: `-race` full suite, lint 0 issues, comment-lint, arch-lint, plimsoll,
coverage 79.9%.

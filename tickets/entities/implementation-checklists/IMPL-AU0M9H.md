---
id: IMPL-AU0M9H
type: implementation-checklist
title: 'Implementation: Relation writes cannot name the source face, so a content-scoped relation on a faced type is unwritable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The source face is threaded from the HTTP address to the store:

1. `acl.RelationSubject.FromFace` + `authz_write.go` passes it to
`decideFromAttrs` instead of a hardcoded zero face, so a content-scoped edge
authorizes against the state it belongs to.
2. `entity.RelationOptions.FromFace` — the write API can now name a tail.
3. `store.RelationWriter.UpdateRelationState`, the faced sibling of
`UpdateRelation`, implemented in all four backends with `UpdateRelation` kept as
a default-tail alias (no caller ripple).
4. `entitymanager`: `CreateRelation`/`UpdateRelation` carry the face;
`DeleteRelationState` added; `getRelationOnFace` replaces the three
`GetRelation` lookups that were default-tail-only.
5. `dataentry`: `applyRelationsModern` takes an `entityRef`, computes the tail
per relation type, and the 422 guard is deleted.

Edge cases handled: an INCOMING edge tails at the peer, so the face is not
applied; an identity-scoped edge stays entity-level on the zero face; the
reconciler's diff is face-scoped so a PATCH to one face cannot delete another's
links.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Store contract: three `UpdateRelationState` cases added to `storetest/states.go`
(the mandatory States suite), mirroring the `DeleteRelationState` trio —
addresses the tail not the triple, zero face matches the unfaced form, and a
miss is `ErrNotFound` without touching the default edge. Verified on **all four
backends**, postgres included.

API level: `TestFacedAddress_PatchWritesTheNamedFace` now asserts the write
succeeds and the edge lands on the addressed face — `assertEdgeTail` /
`assertNoEdgeTail` query by BARE id deliberately, so an assertion sees where an
edge actually landed rather than only where it was expected.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The reported scenario, run as a throwaway test against the faced fixture:

```
published PATCH -> 200        (was 422 with impossible advice)
draft clear     -> 200
published edge survived a draft-side clear
```

The second step is the one that matters beyond the report: clearing the draft's
links leaves the published face's intact, so the faces are genuinely isolated
rather than merely writable.

Before the fix the same scenario gave `422 face_relations_unsupported`, and the
address its detail recommended gave `404`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `DeleteRelationState` is the precedent for every new faced
method (same positional face argument, same "tail is identity, not a filter"
rationale); `EntitySubject.Face` is the precedent for the ACL field including
its backward-compatibility argument.

**Security.** The ACL change is the load-bearing part. `GrantsVerbOnState`
treats `*` as covering only the default face, so a faced relation write now
requires an explicit `type@face` grant — a principal granted `policy@draft`
cannot write the published face's edges. This tightens rather than relaxes:
before, relation writes authorized against a hardcoded zero face. Existing
grants keep their meaning because identity-scoped and faceless writes still pass
the zero face.

`getRelationOnFace` fails closed — a query error is returned as-is, never
flattened into not-found, because the create path's not-found branch decides
whether a write proceeds.

Gates: `just arch-lint`, `just comment-lint`, `just plimsoll` (pins bumped with
reasons on the four store types and `Manager`), `golangci-lint` all clean; full
`./internal/...` suite and `-race` pass.

**Known limitation, filed not hidden:** a faced relation *delete* records no
version — `recordRelationVersion` skips state-tailed edges because the
synchronous path can only address the default lineage. The skip fails safe (no
history rather than another lineage's history) and pre-dates this work, but this
bug made it reachable from a client. TKT-JAROC3, with a pointer at the skip
site.

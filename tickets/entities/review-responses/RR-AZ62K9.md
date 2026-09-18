---
id: RR-AZ62K9
type: review-response
title: 'Relation-create body target was never read-gated: an existence oracle, and a link to a row the caller was never shown'
finding: 'handleV1CreateRelation read-gated only the PATH entity; the body target ({"id": ...}) was never gated anywhere, and the manager authorizes on FromType/FromID only. Demonstrated: a principal who may read tickets but not features got 201 when naming FEAT-001 (exists, unreadable) as the target and 422 "target entity not found" when naming FEAT-999 (absent). Two distinguishable responses is an existence oracle for any id the caller cannot read, and the 201 wrote an edge to a row they were never shown. Pre-existing on develop (git show confirms the body target was never gated), but TKT-R4BMJM put weight on it: RR-X3O9RR''s fix addressed the link_as=from create FROM the new entity with the peer in the BODY, specifically so a prefix-less peer id could be linked. That moved the peer out from under the path gate, and on that route the path gate is near-vacuous because the caller just created the entity it names.'
severity: critical
resolution: 'Read-gate the body target too, before the affordance checks. The target''s type is resolved from the STORE rather than from its id prefix — a prefix-derived type is the same fragile lookup RR-X3O9RR removed, and would have failed for exactly the prefix-less ids that fix enabled. A target that does not exist and one the caller may not read both collapse to a 404 carrying the shared entityNotFoundTitle, so they are byte-identical. Pinned by TestACLWrite_RelationTargetIsReadGated, which asserts BOTH arms: the hidden target 404s, and its response is identical to the nonexistent one. Only the pair is the property — a 404 on the hidden arm alone would be satisfied by an implementation that 404s everything.'
status: addressed
---

## Finding

`handleV1CreateRelation` read-gated only the **path** entity. The body target
(`{"id": ...}`) was never gated, and `manager.CreateRelation` authorizes on
`FromType`/`FromID` only.

Demonstrated with a principal who may read tickets but not features:

| Target | Result |
| --- | --- |
| `FEAT-001` (exists, unreadable) | **201** — edge written |
| `FEAT-999` (absent) | **422** `target entity not found` |

Two distinguishable responses is an existence oracle for any id the caller
cannot read. And the 201 wrote an edge to a row they were never shown.

**Pre-existing** on `develop` — `git show` confirms the body target was never
gated. But this ticket put weight on it: RR-X3O9RR's fix addressed the `link_as:
from` create *from the new entity* with the peer in the **body**, specifically
so a prefix-less peer id could be linked at all. That moved the peer out from
under the path gate — and on that route the path gate is near-vacuous, because
the caller just created the entity it names.

## Resolution

Read-gate the body target as well, before the affordance checks.

The target's type is resolved from the **store**, not from its id prefix. A
prefix-derived type is the same fragile lookup RR-X3O9RR removed, and it would
have failed for exactly the prefix-less ids that fix enabled — reintroducing one
bug while closing another.

A target that does not exist and one the caller may not read both collapse to a
404 carrying the shared `entityNotFoundTitle`, so they are byte-identical.

`TestACLWrite_RelationTargetIsReadGated` asserts **both arms**: the hidden
target 404s, *and* its response is identical to the nonexistent one. Only the
pair is the property — a 404 on the hidden arm alone would be satisfied by an
implementation that 404s everything, which is the shape the repo's own guidance
warns about.

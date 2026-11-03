---
id: PLAN-NETME1
type: planning-checklist
title: 'Planning: Enforce relation cardinality at write time and add an atomic replace operation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `max_outgoing` and `max_incoming` were checked only by `analyze`. A re-point was two writes that could leave zero or two edges.

**Scope:**
- In: refuse new edges over a bound on every write path; one-transaction re-point for bounded relations through the relations PATCH (`data` or `add` plus `remove`); grandfather existing data over a bound.
- Out: bulk loaders (`rela import`, data migrations) and restore of a soft-deleted entity stay unbounded; `analyze cardinality` reports them.

**Acceptance Criteria:**
1. A second `add` on a `max_outgoing: 1` relation is refused with 422 `cardinality_exceeded` that names the relation and the limit. Test: TestPatchRelations_AddOverMaxOutgoingIs422, TestCreateRelation_RejectsSecondEdgeOverMaxOutgoing.
2. A replace leaves exactly one edge, also when the old edge is gone. Test: TestReplaceRelations_RepointsToExactlyOneEdge, TestReplaceRelations_CreatesWhenRemovedEdgeIsGone.
3. A failed replace keeps the original edge. Test: TestReplaceRelations_RefusedCreateKeepsOriginalEdge, TestPatchRelations_RepointToMissingTargetKeepsEdge.

## Research

- [x] For larger features: run `/research` to create a structured research doc (RES-8CKUNJ)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: an internal metamodel and view feature; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: done once for the whole design in TKT-ZNFGNJ and RES-8CKUNJ)
- [x] Reviewed relevant rela concepts for prior art (metamodel-types, views, data-entry-ui)

**Research Doc:** RES-8CKUNJ (option D: status entity plus relation paths).

**Existing Solutions:** `internal/schema/cardinality.go` already computed the bounds for `analyze`. `relations_modern.go` applied PATCH deltas edge by edge. The store already offers transactions on Postgres and SQLite.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Check bounds in entitymanager (`cardinality.go`) on CreateRelation, cascade writes and copy. Add `ReplaceRelations`: one store transaction that removes the named edges and creates the new ones, counting capacity without the removed edges. The relations PATCH routes every bounded relation through it. CalDAV moves on a bounded membership use it too.

**Alternatives:** A separate `replace` op in the PATCH was dropped: `data` or `add` plus `remove` in one PATCH already express a re-point, and the form sends deltas. Rejecting data that is already over a bound was rejected because it would lock users out of hand-edited files.

**Dependencies:** store.Tx, entitymanager, dataentry relations PATCH, CalDAV write path, relation version recorder.

**Files to modify:** internal/entitymanager/{cardinality,manager,cascadehost,copy_apply,version_hook,softdelete}.go, internal/dataentry/{relations_modern,write_handler,caldav_write,app}.go, internal/appbuild/appbuild.go, GUIDE-concepts and GUIDE-metamodel.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Relations PATCH and POST bodies from the API, MCP, CLI, Lua and CalDAV. Relation keys are resolved against the schema (allowlist). Unknown keys and targets are refused.

**Security-Sensitive Operations:** Deleting edges in a replace. The ACL is checked per removed edge from a fixed list inside the transaction, so a caller can only remove edges it may delete. The 422 names only the caller's entity and the bound.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each AC maps to the tests named above. Integration: dataentry handler tests run the full PATCH path on a real manager and store; Postgres tests run with `-tags postgres`.

**Edge Cases:** old edge already gone; inverse-side re-point; data already over the bound; two creates within `max_outgoing: 2`; key named twice; disallowed target type; CalDAV move, stale edit and DELETE; content-scope relations counted per face.

**Negative Tests:** add over bound (422); full linkage over bound refused before any write; missing target leaves the edge; denied remove writes nothing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): m

**Risks:** Every write path changes. Mitigation: one check in entitymanager that all paths share, tests per path, and grandfathering so existing data stays editable. Non-atomic file and memory stores create before they remove, so a failure leaves an extra edge, never a missing one.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/concepts.md (cardinality on write, re-point, CalDAV) and docs/metamodel.md (`max_outgoing`, `max_incoming` rows).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was settled in TKT-ZNFGNJ and RES-8CKUNJ and checked with the working demo in examples/relation-status-demo; two code-review rounds covered the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design-review findings; code-review findings are tracked as review-responses on the ticket)

**Design Review Findings:** None. Code-review findings: see the review checklist.

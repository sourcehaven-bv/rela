---
id: PLAN-RZ50SE2
type: planning-checklist
title: 'Planning: Relation filter controls match on target id instead of display title'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `filter[<relation>]` and `filter[<relation>][ne]` on list,
kanban, scope navigation and export (all share the list pipeline) match a target
id; title values keep working, deprecated; id filters push down to the store
when safe; the SPA selector commits ids. Out: removing title matching; new
operators; multi-value relation filters.

**Acceptance Criteria:**
1. Two same-titled targets filter separately: `TestV1ListRelationFilter_ByID` (PRJ-001 and PRJ-003 both "Apollo").
2. Renaming a target does not change filter results: `TestV1ListRelationFilter_ByIDSurvivesRename`.
3. Legacy title values still match: `TestV1ListRelationFilter_ByID` legacy case and the existing `TestV1ListRelationFilter`.
4. An unreadable id is indistinguishable from a missing one: `TestV1ListRelationFilter_UnreadableIDIsNotAnOracle`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: covered by RES-8CKUNJ option D item 7)
- [x] ~~Searched for existing libraries~~ (N/A: internal query path)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal query path)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-8CKUNJ

**Existing Solutions:**
- `appbuild/conditionlower.go` gates a `related(...)` hop with `GateTraversal` and appends it to `GraphQuery.Related`; the pushdown follows the same shape, including Denied and Negate handling.
- `acl.TraversalHop.EndpointIDs` already restricts a hop to ids.
- `visibleReader.servedIDsErr` resolves ids through the read gate in the request world.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `gateAppListQuery` resolves every relation filter value once, after the field gate, with one `servedIDsErr` call. A served id is an id target (id, served type); anything else is a legacy title target. Stored on `gatedListQuery.relationTargets` by query key. A lookup error is returned.
2. `matchRelationFilterMany` takes the target: id mode compares the neighbour id; title mode keeps the redacted display-title match and logs a deprecation warning without the value.
3. `planListPushdown` accepts an id-mode relation key (eq/ne, one value, no `[]`, identity-scoped relation, target type unfaced or allow-all). `pushdownPlan` gates one hop per key through the traversal gate and ANDs it into `GraphQuery.Related` (`Negate` for ne). Denied: eq is empty, ne adds nothing. Any other error or a negated gate predicate declines to the Go path.
4. `EntityTargetSelect` commits the candidate id, labels with "Title (ID)", and no longer collapses duplicate titles.

Alternatives: a new `filter[rel][id]` operator (rejected: two shapes for one
control, and old URLs would still need the title path); title matching dropped
outright (rejected: breaks saved URLs).

**Files to modify:** internal/dataentry/api_v1.go, list_pipeline.go,
listfieldgate.go, listpushdown.go, relation_filter_id_test.go (new),
listpushdown_test.go; frontend/src/components/common/EntityTargetSelect.vue
(+test), lists/FilterBar.vue (+test); e2e/tests/list.spec.ts; docs-project
data-entry guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The filter value is caller-supplied. It is only
ever compared: as an id through the gated resolver, or as a title against
redacted served neighbours. The relation key is still allowlisted by configured
filter controls.

**Security-Sensitive Operations:** Threat model in the design: mode choice uses
the gated resolver, so an unreadable id behaves like a missing one; the pushed
hop is gated by `GateTraversal` (row gate and world); content-scoped relations
and faced, row-gated target types stay on the Go path (RR-DQFDRXO); a lookup
fault is a 500, not a widened `ne` (RR-L6LJZQS); the log omits the value
(RR-EVMC12L).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Handler tests in `relation_filter_id_test.go` per acceptance
criterion; pushdown plan tests that an id filter is pushed and that a title
value, a content-scoped relation and a faced target decline; SPA unit tests for
the selector; e2e list spec selects by id.

**Edge Cases:** Empty value (no constraint); unknown id (matches nothing, ne
matches all); value that is both an id and a title (id wins); paging with an id
filter; incoming and outgoing directions.

**Negative Tests:** Unsupported operators still 400; unreadable id equals
missing id for eq and ne.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Pushdown and Go path disagreeing; mitigated by conservative
eligibility and tests over both paths. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via docs-project GUIDE-data-entry): relation filter values are ids; titles deprecated.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DQFDRXO (significant), RR-L6LJZQS (minor),
RR-EVMC12L (minor). All folded into the approach.

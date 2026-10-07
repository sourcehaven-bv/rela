---
id: PLAN-K0RIBS
type: planning-checklist
title: 'Planning: List group_by on a single-valued relation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** List `group_by` accepts only a property, so a list cannot show tasks in sections by their status relation.

**Scope:**
- In: `group_by: {relation, offered_by, order_by}` with sections from the target entities; section Add prefills the relation; the same column resolution as the kanban.
- Out: grouping on multi-valued relations.

**Acceptance Criteria:**
1. A grouped list and the kanban for the same anchor show the same sections in the same order. Test: both use useRelationColumns; checked on the demo `lijst` tab against the board.
2. Creating from a section links the new entity to that section's target. Test: demo `/list/taken`, Add in a section.

## Research

- [x] For larger features: run `/research` to create a structured research doc (RES-8CKUNJ)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: an internal metamodel and view feature; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: done once for the whole design in TKT-ZNFGNJ and RES-8CKUNJ)
- [x] Reviewed relevant rela concepts for prior art (metamodel-types, views, data-entry-ui)

**Research Doc:** RES-8CKUNJ (option D: status entity plus relation paths).

**Existing Solutions:** Kanban `columns_from` resolution in useRelationColumns.ts; existing property grouping in useListGrouping.ts and validate_groupby.go.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Extend the group_by config with a relation form, validated by the shared `validateRelationColumns`. In the SPA, useListGrouping takes its sections from useRelationColumns and sets the relation in the section create prefill.

**Alternatives:** A separate list-only resolution was rejected so lists and boards cannot disagree.

**Dependencies:** TKT-KJ3Q07 (column resolution).

**Files to modify:** internal/dataentryconfig/{groupby,validate,validate_groupby}.go, frontend/src/composables/useListGrouping.ts, useRelationColumns.ts, frontend/src/components/lists/EntityList.vue, InlineCreateFormModal.vue, utils/listParams.ts, the demo.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml config validated at load, same rules as `columns_from`.

**Security-Sensitive Operations:** None new; create goes through the existing create path.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Config tests in validate_groupby_test.go and validate_columns_from_test.go; section behaviour checked on the demo.

**Edge Cases:** entity with no target or a target not offered (Other section); empty sections follow `groups`.

**Negative Tests:** multi-valued relation or wrong start type: config validation error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): m

**Risks:** Sections and columns could drift. Mitigation: one shared composable.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md: `group_by` on a relation.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was settled in TKT-ZNFGNJ and RES-8CKUNJ and checked with the working demo in examples/relation-status-demo; two code-review rounds covered the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design-review findings; code-review findings are tracked as review-responses on the ticket)

**Design Review Findings:** None. Code-review findings: see the review checklist.

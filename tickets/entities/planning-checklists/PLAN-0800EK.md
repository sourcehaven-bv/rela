---
id: PLAN-0800EK
type: planning-checklist
title: 'Planning: Kanban columns from a single-valued relation (columns_from)'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** A kanban takes its columns only from an enum property, so every parent shares one global set of columns.

**Scope:**
- In: `columns_from: {relation, offered_by, order_by, style_from}`; drag re-points the relation; per-column create with the target prefilled; a trailing Other column shown only when not empty.
- Out: swimlanes from a relation; column colour from relation paths (replaced by `style_from`).

**Acceptance Criteria:**
1. An initiative page with an `offers_status` order shows exactly those columns in that order, plus Other when needed. Test: demo initiative boards.
2. Dragging a card changes its status edge and nothing else. Test: KanbanView.columnsFrom.test.ts "re-points a dropped card with a full linkage".
3. Reordering the anchor's `offers_status` reorders the columns. Test: columns follow `_order_out` in useRelationColumns.
4. Add in a column prefills that target. Test: "offers Add in each column and prefills the column target".

## Research

- [x] For larger features: run `/research` to create a structured research doc (RES-8CKUNJ)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: an internal metamodel and view feature; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: done once for the whole design in TKT-ZNFGNJ and RES-8CKUNJ)
- [x] Reviewed relevant rela concepts for prior art (metamodel-types, views, data-entry-ui)

**Research Doc:** RES-8CKUNJ (option D: status entity plus relation paths).

**Existing Solutions:** The demo prototype on `demo/relation-backed-status`; the full-linkage PATCH in `relationsPatch.ts`; enum column validation in `validate.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Validate `columns_from` in dataentryconfig (`validateKanbanColumnsFrom`, shared `validateRelationColumns`). Resolve columns in the SPA in `useRelationColumns.ts`: anchor targets in `_order_out` order with `offered_by`, otherwise all targets by `order_by`. A drop sends a full linkage for the relation, which the server re-points in one transaction (TKT-65LVAK).

**Alternatives:** `column_relation` as a single key was rejected for a block that also carries the anchor relation and order. Colour through a `has_status.kleur` path was replaced by `style_from`, which reuses `styles:`.

**Dependencies:** TKT-65LVAK for the one-transaction re-point; orderable relations (`_order_out`).

**Files to modify:** internal/dataentryconfig/{config,validate,groupby,validate_groupby}.go, frontend/src/composables/useRelationColumns.ts, frontend/src/views/KanbanView.vue, frontend/src/components/forms/DynamicForm.vue, frontend/src/utils/styleColors.ts, the demo project.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml config, validated at load: the relation must exist, start at the card type and be single-valued; `offered_by` must start at the anchor type; `style_from` must be an enum property of the target.

**Security-Sensitive Operations:** None new. The drop uses the existing relations PATCH with its ACL checks.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Each AC maps to the test named above; config validation in validate_columns_from_test.go.

**Edge Cases:** card with no target or a target the anchor does not offer (Other column); target without `style_from` value (uncoloured); template switch after a prefill.

**Negative Tests:** multi-valued relation, wrong start type, unknown `style_from` property: config validation errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): l

**Risks:** Drag on a bounded relation depends on TKT-65LVAK. Mitigation: built on top of it. Other column could hide cards. Mitigation: it is shown whenever it has a card.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md: columns from a relation, Other column, per-column create, `style_from`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was settled in TKT-ZNFGNJ and RES-8CKUNJ and checked with the working demo in examples/relation-status-demo; two code-review rounds covered the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design-review findings; code-review findings are tracked as review-responses on the ticket)

**Design Review Findings:** None. Code-review findings: see the review checklist.

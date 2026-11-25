---
id: PLAN-YCRS8Y5
type: planning-checklist
title: 'Planning: List toggle widget'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `widget: toggle|switch` with `on`, `off`, `other` on list
columns; load checks; SPA renders a checkbox or switch and PATCHes the property.
Out: dashboard card and view section columns (refused at load), relation
columns, kanban cards.

**Acceptance Criteria:**
1. A boolean column with `widget: toggle` renders a checkbox; a click PATCHes true/false (EntityList test).
2. An enum column with `on: done, off: open` writes those values (EntityList test).
3. An enum with other values and no `other:` fails to load (validate test).
4. `other: disabled|hidden|on` controls the rows holding another value (EntityList test).
5. A row whose `_fields` marks the property read-only, or whose transition to the target value is not allowed (`_fields.options`), renders the control disabled (EntityList test); the server still refuses the PATCH.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small, follows the editable-table pattern)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Editable table cells (TKT-4Q2EIVZ,
internal/dataentry/sections.go withRowEditData); list rows already carry
`_fields` from verdicts primed once per page (entity_read_handler.go
primeVerdicts); list-only column keys follow busy_column.go. No library needed:
RlCheckbox / RlSwitch exist in the SPA.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `Widget`, `On`, `Off`, `Other` to ListColumn. Load
check `checkListToggleColumn`: plain property column (no relation, face, busy,
link, render); boolean takes no on/off; enum needs on and off, distinct declared
values, and `other:` exactly when other values exist; other types refused.
Refuse the keys on dashboard and view section columns. Classify the keys in
configedit. SPA: a toggle cell reads `_fields[prop]` (writable, options) and
`_actions.update`, PATCHes `{prop: target}` through the existing api client,
updates the row optimistically and reverts on error. Alternative rejected: a
dedicated toggle endpoint (would duplicate the PATCH authorization).

**Files to modify:**
internal/dataentryconfig/{config.go,busy_column.go,toggle_column.go,validate.go},
internal/configedit/allowlist.go, frontend/src/types/config.ts,
frontend/src/components/lists/EntityList.vue, docs-project (list column docs),
examples/todo-app/data-entry.yaml.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Config (operator) checked at load against the
metamodel. User click sends a normal PATCH; the server re-authorizes field
permission, enum validity and transition guards.

**Security-Sensitive Operations:** The write. It goes through entitymanager like
every PATCH; `_fields` is a UI hint only. Threat model:
.ignored/threat-TKT-M0AT73B.md.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Go table tests for the load check; vitest for rendering,
PATCH payload, disabled state and revert; existing list budget test covers
per-page `_fields`.

**Edge Cases:** unset value (off); value outside on/off with each `other`;
hidden field (no value: cell shows nothing); multi-value enum refused.

**Negative Tests:** on/off not in enum; on == off; on/off on a boolean; widget
on relation/face/busy column; widget on a dashboard card column; unknown widget
or other value.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Accidental flips on a dense list: the control has an accessible name
and stops row click propagation. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** data-entry list column reference in docs-project;
examples/todo-app.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-Z7M29ZG, RR-L5CJELL, RR-BBAQ6GC (all addressed:
row replaced from the PATCH response, first column refused, a click on a checked
box writes off).

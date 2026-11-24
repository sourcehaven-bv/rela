---
id: PLAN-KPG6IQE
type: planning-checklist
title: 'Planning: Data-entry config fixes: one colour list, board condition, reserved keys, select widget error'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: one colour list (`ColorNames`) for styles, list groups, calendar,
gantt and sankey; kanban `condition:` via `kanban_id`; reserved-key error naming
key, action and lists; deprecation error for `widget: select`/`multi-select` on
form property fields; docs in docs-project. Out: relation `widget:` (still a
choice), channel and view-section widgets, examples/todo-app.

**Acceptance Criteria:**
1. `amber`, `violet`, `slate`, `gray` load in `styles:` and every bare name loads in groups and calendars (TestColorNames_EverySurface); SPA maps every name (TestColorNamesMatchSPA).
2. A board with `condition:` loads and `?kanban_id=` filters its cards (TestValidateConfig_KanbanCondition, TestViewCondition_KanbanFiltersTheBoard); naming both views is a 400 (TestViewCondition_ListAndKanbanIDIsBadRequest).
3. Reserved key error names key, action, list (TestValidateConfig_ReservedListKey); keys match the SPA (TestReservedListKeysMatchSPA).
4. Select widget error names form, field and line to remove (TestValidateConfig_SelectWidgetDeprecated).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: four small config fixes)
- [x] ~~Searched for existing libraries~~ (N/A: internal config validation)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `ParseColor` and hue aliases in colors.go; `list_id`
condition path in listpushdown.go/viewcondition.go; kanban conditions already
compiled by conditionlint.CompileViewConditions; `reservedListKeys` in
validate.go.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `ColorNames` map name→hue replaces bareHues, hueAliases,
ValidGroupColors, ValidCalendarColors; `ParseNamedColor` for bare-only surfaces;
SPA `NAMED_COLORS` + `namedColor`/`namedColorVar`. `requestViewCondition`
resolves `list_id` or `kanban_id` inside `resolveListNarrowing`, so the board
reuses pushdown, redaction and ACL. Alternative of normalising names at load
rejected: it changes wire values for no gain.

**Files to modify:**
internal/dataentryconfig/{colors,config,validate,validate_groupby,groupby,sankey}.go,
internal/dataentry/{viewcondition,listpushdown,api_v1,livepath,requeststats}.go,
frontend styleColors.ts, calendar chip/legend, SankeyView, useListGrouping,
KanbanView, types; docs-project GUIDE-data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Colour names and keys come from operator config,
checked against allowlists at load. `kanban_id` comes from the request; it is an
id looked up in config, never an expression. Unknown id means no condition, as
for `list_id`.

**Security-Sensitive Operations:** Threat model: a condition only narrows the
ACL-scoped set, so a forged or omitted `kanban_id` cannot widen a read. The
condition is evaluated on the redacted row or lowered only when the fields are
visible (same code as lists), so no hidden-field oracle. Both ids at once is a
400 so no view is picked silently. Colour names reach CSS only through fixed
token maps, never as raw strings. Error messages name config only (not secret
per CLAUDE.md).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; the kanban test runs the HTTP
handler with the production compiler.

**Edge Cases:** unknown kanban id (unconstrained); list id equal to a kanban id
(does not cross); shade on a bare-only surface (rejected); operator colour named
like a new alias (rejected as reserved).

**Negative Tests:** `pink`, both view ids, every reserved key,
select/multi-select in flat and wizard forms.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Existing configs with form `widget: select` stop loading (scanned all
tracked data-entry.yaml: none). An operator colour named
`amber`/`violet`/`slate` is now reserved. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project GUIDE-data-entry.md (colour names, board
condition, reserved keys, widget deprecation); docs/data-entry.md regenerated.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No critical or significant findings. Minor: sending
`kanban_id` on every board read adds a cache-key part; accepted, it mirrors
`list_id`.

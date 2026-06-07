<!-- @managed: claude-workflow v1 -->
---
id: IMPL-UD7YR
type: implementation-checklist
title: 'Implementation: Route view-side per-field rendering through widget registry'
status: done
---

## Development

- [x] Unit tests written for new code (widgets.test.ts, wrapperWidgets.test.ts, viewRouting.test.ts, InaccessibleField.test.ts — +110 tests across both rounds)
- [x] Integration tests written (existing FieldRenderer.test.ts + Badge.test.ts exercise the full delegation path)
- [x] Happy path implemented (8 property widgets gained display branches; cards/list/properties delegate via registry)
- [x] Edge cases from planning handled (null/undefined values, empty arrays via em-dash, long arrays via comma-join fallback, inaccessible short-circuit, array-into-SelectWidget defensive guard)
- [x] Error handling in place (Badge gray fallback, formatDate/formatValue raw-string fallback, SelectWidget array warn, registry unknown-widget warn)

## Test Quality

- [x] Using fixture builders or factories for test data (per-widget test mounts; viewRouting test uses ViewSectionField fixtures)
- [x] No hardcoded values in assertions when object is in scope (Badge/widget reference comparisons by identity)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation in these tests)
- [x] ~~Property comparisons use original object~~ (N/A: tests assert emitted events and DOM shapes)

## Manual Verification

- [x] Feature manually tested end-to-end (browser smoke against the tickets project)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (empty enum field shows em-dash; cards show colored badges; Required Concepts list with "Server"/"Stable" rendering)

**Verification Evidence:**

Built frontend + rela-server, ran against `tickets/` project, drove with browser:

- Feature view (FEAT-72NR1): Required Concepts list shows "Server"/"Stable"/"Core"/"Stable" badges with proper schemaStore colors; Implementation Tickets table (out of scope, unchanged) shows status badges; entity properties show kind/priority/effort/status badges.
- Ticket view (TKT-MZSIJ): properties section delegates through PropertyDisplay → widget registry; cards/list sections render relations correctly.
- TKT-IHCY7 surfaced a *pre-existing* data bug (literal `\n` in YAML body created by the MCP back in the first session) unrelated to this refactor; logged for separate cleanup.

Automated gate:

- `npm run typecheck`: clean
- `npm run lint`: 0 errors (77 pre-existing console warnings, untouched files)
- `npm run test:run`: 950 passed (was 938 before code-review fixes; +60 new tests across this ticket)
- Coverage: widgets dir maintained; the one `coverage:check` violation (`src/stores/schema.ts`) is the same pre-existing flake reported on TKT-MZSIJ.

## Quality

- [x] Code follows project patterns (registry + viewRouting helper mirrors the `consumer-side interface` pattern from CLAUDE.md; required-prop strictness matches FieldRenderer's evolution in TKT-MZSIJ)
- [x] Checked for DRY opportunities (viewFieldRoutingHint extracted to widgets/viewRouting.ts; InaccessibleField extracted to common/; fieldRowsFor centralizes the per-row precompute)
- [x] No security issues introduced (no new v-html; widget routing maps names to component refs; existing escaping preserved)
- [x] No silent failures (SelectWidget array, registry unknown-widget, Badge missing property — all surface visibly or fall back to documented defaults)
- [x] No debug code left behind

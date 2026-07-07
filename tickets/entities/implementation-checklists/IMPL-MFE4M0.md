---
id: IMPL-MFE4M0
type: implementation-checklist
title: 'Implementation: Enum values support a display label/title for better UX on snake_case values'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — `_schema` API round-trip + live-server end-to-end verification
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed) — N/A control flow: labels are optional/tolerant by design; no new error paths

## Test Quality

- [x] Using fixture builders or factories for test data — seeded metamodel/store fixtures; existing `newTestAppV1`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (live `rela-server`, `_schema` + OpenAPI + entity create)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela-server`, ran against a scratch project with a labeled custom type
(`ticket_status`) and a labeled inline list-enum (`tags`):

- **AC1 (custom type):** `/_schema` → `types.ticket_status.labels = {in_progress: "In Progress", wont_fix: "Won't Fix"}`; the `status` property (custom-type-backed) inherited those labels on the wire.
- **AC2 (inline enum):** `tags` property carried `labels = {needs_review: "Needs Review"}`.
- **AC3/AC4 (badge + fallback):** covered by `Badge.test.ts` — label shown when configured, color class still derived from the raw value, no-label value falls back to raw + capitalize.
- **AC5 (back-compat):** `title` (non-enum) emitted no `labels` key (omitempty); `TestParse_EnumLabels` asserts a label-less enum parses with nil Labels; `TestV1SchemaEnumLabelsOmittedWhenAbsent` asserts no `labels` key emitted.
- **AC6 (validation value-based):** created entities via API stored raw `in_progress`/`wont_fix`; validation unchanged.
- **AC7 (kanban/filter/edit surfaces):** `KanbanView.test.ts` (column header resolves enum label, explicit config label still wins), `AdHocFilterMenu.test.ts` (picker shows label, applies raw value), FilterBar option text via `optionText`.
- **OpenAPI reach guard:** live `_openapi.json` `status` enum = `[backlog, in_progress, wont_fix, done]` (no labels); `TestGenerator_EnumLabelsDoNotLeakIntoEnum` pins it.

Precedence (RR-UAV796): `TestV1SchemaEnumLabels` asserts a custom-typed property
inherits the custom type's labels and that an inline `labels` map on such a
property is ignored.

Note: browser-based visual check was not run (Chrome extension not connected);
the render logic is covered by the widget/component unit tests above.

## Quality

- [x] Code follows project patterns (consumer-side lookup via store getter mirrors existing `styles`; shared `toV1CustomType` helper avoids drift)
- [x] Checked for DRY opportunities — extracted `toV1CustomType` (two serialization sites), `labelsForProperty`/`getEnumLabel` store getter (reused by Badge, SelectWidget, MultiSelect, Kanban, AdHocFilter)
- [x] No security issues introduced — labels rendered via `{{ }}` text interpolation (escaped), no `v-html`; escaping test in `Badge.test.ts`
- [x] No silent failures
- [x] No debug code left behind

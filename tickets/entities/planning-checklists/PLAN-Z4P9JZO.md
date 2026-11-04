---
id: PLAN-Z4P9JZO
type: planning-checklist
title: 'Planning: Edit owned and linked items inline on the parent page'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

After the design review the ticket was split (RR-CPB5PXW). This ticket covers
editable table cells and section totals. Row actions, collapsible cards and row
prose editing moved to their own backlog tickets.

In scope:

1. Editable table cells. A column in a view section's `columns:` or
`columns_by_type:` takes `render: input`. Section-level `render:` stays inert on
tables (RR-F35K7DL), so no existing config becomes editable on upgrade. Table
rows carry `_props` and `_fields` only when one of their columns is editable. A
cell renders through the widget registry in edit mode via `InlinePropertyValue`,
the same as list and cards rows (RR-3XLCZ00). One autosave per row, through
`runSessionWrite`.
2. The autosave host logic of `SectionEditForm` moves into a composable
(`useSectionAutoSave`) used by both `SectionEditForm` and the new table row
cells, so the invariants exist once (RR-8669P8G). A row apply path covers `rows`
and `groups[].rows`, and cells read `_props` before `values`.
3. Section totals. A table section takes `totals:`, a list of
`{property, label, emphasis}` naming properties of the view's entry type
(typically `rollup:` properties). Only that config goes on the wire; the SPA
reads the values from the entry, which is already gated with roll-ups merged
(RR-PIG9KY8). After a row save the SPA refreshes the entry (coalesced, only when
the section has totals). A hidden or absent value leaves its line out.

Out of scope:

- Row actions (delete with Undo, remove link): TKT-48ZHQZ3.
- Collapsible cards: separate backlog ticket.
- Row prose editing: separate backlog ticket.
- Version tokens on rows: cell saves are unguarded until their first save
returns tokens, as list rows are today (RR-8I32H8V, TKT-MK9NJB).
- Totals refresh after another user's edit (entity:changed only refreshes
the page's own type).
- Adding rows inline in a table.

**Acceptance Criteria:**

1. A table column with `render: input` on a writable property edits in place
with the property's widget; the value saves after the debounce, the row shows
the AutoSaveIndicator, and a reload shows the new value.
2. A cell renders as display when the field verdict is not writable, when
the column has no `render: input`, or when the section has more than 100 rows
across all groups (RR-PA3KBWR).
3. Section-level `render: input` on a table still renders display and still
produces the inert-key warning.
4. A table section with `totals:` shows the entry's values under the table
with `RlTotals`, the emphasis line bold, and they change after a row edit. A
total the reader cannot see is left out.
5. Rows carry `_props`/`_fields` only when a column is editable; hidden
properties are stripped as in list rows.
6. Config load refuses (RR-JXTTP5L): `render` on list, dashboard or nested
columns; `render: input` with `link:`, `relation:`, `face:` or `busy:`; `totals`
on a non-table section, on a side-panel section, or naming a property the entry
type does not declare.
7. Editing the `group_by` property of a row moves the row only on the next
view load.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the Payload gap analysis and the Storybook mockup in this
session served as research.

**Existing Solutions:**

- Payload CMS array/blocks fields (the source of the gap analysis): blocks
are collapsible rows with a type picker; arrays are tables.
- Row inline edit: `rowShouldRouteToInlineEdit` and `SectionEditForm`
(`frontend/src/components/entity/sectionEditFields.ts`, `EntityDetail.vue`
cards/list arms).
- Autosave: `frontend/src/composables/useAutoSave.ts` (PATCH with per-field
preconditions).
- Undo toast: `frontend/src/composables/useBulkDelete.ts`; restore endpoint
`POST /{plural}/{id}/restore`.
- Unlink: `deleteRelation` in `frontend/src/api/entities.ts`.
- Totals: metamodel `rollup:` properties (FEAT-W2ZXK9).
- Library parts built for this: `RlInlineEdit numeric`, `RlTotals`,
`RlAccordion` with `#actions`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Go:
- `dataentryconfig`: `ListColumn.Render`; `ViewSection.Totals
[]SectionTotal{Property, Label, Emphasis}`; validation for AC6.
- `dataentry/sections.go`: `buildTableSection` sets `Props` and
`FieldVerdicts` on rows of a column set with an editable column, using
`copyVisibleProperties` and `computeFieldAffordances` (verdicts primed once per
section by `primeVerdicts`).
- `apiwire/v1`: `ViewRow._props`, `ViewRow._fields`, `ViewColumn.render`,
`ViewSection.totals`; OpenAPI schema.

SPA:
- `composables/useSectionAutoSave.ts`: extracted from `SectionEditForm`.
- `components/entity/EditableRowCells.vue`: renders a row's `<td>`s
(fragment root) and its actions cell with the indicator.
- `EntityDetail.vue`: wiring, row apply path, cap over groups, totals block
and entry refresh.
- i18n en/nl for any new strings.

Alternatives rejected:
- `RlInlineEdit` per cell: bypasses the widget registry.
- A second autosave host for rows: duplicates invariants that caused
BUG-NSGD6OE and BUG-04V034W.
- Client-side sums: duplicate roll-up logic and its ACL folding.
- Values copied into the section's totals: the entry already carries them.

**Files to modify:**

- internal/dataentryconfig/config.go, validate.go (+ tests)
- internal/dataentry/sections.go, sections_nested.go, wire conversion (+ tests)
- internal/apiwire/v1/responses.go, internal/openapi/schemas.go
- frontend/src/api/views.ts
- frontend/src/components/forms/SectionEditForm.vue,
composables/useSectionAutoSave.ts, components/entity/EditableRowCells.vue,
EntityDetail.vue (+ tests)
- e2e/tests/fixtures.ts, e2e/tests/table-inline-edit.spec.ts
- docs-project data-entry guide

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Cell values: same PATCH path and server validation as forms.
- Config keys: validated at load (allowlisted display modes, known
properties).

**Security-Sensitive Operations:**

- Table `_props` go through the same visibility redaction as the entity
arms; field verdicts decide editability, the server still enforces writes.
- Totals read from the gated, per-principal entry; never cached.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: vitest `EditableRowCells` (edit emits a debounced PATCH through
runWrite; indicator shows); e2e edit a quantity, reload, value persists.
- AC2: vitest display arm for non-writable verdict and plain column; vitest
for the cap counted over groups.
- AC3: existing inert-warning test kept green; e2e fixture has no
section render on the table.
- AC4: vitest totals block reads entry values, skips absent; e2e total
changes after a line edit.
- AC5: Go test in sections_test for `_props`/`_fields` presence and hidden
property stripping.
- AC6: table-driven validation tests.
- AC7: vitest row apply keeps the row in its group.
- Existing `SectionEditForm` tests stay green after the extraction.

**Edge Cases:**

- Cap above 100 rows across groups: display only.
- Unmount mid-debounce (navigation, reorder reload): flushed by the
composable's unmount hook.
- `columns_by_type` groups with different editable columns: per group.
- Totals refresh failing: logged as an error toast, values left as they are.

**Negative Tests:**

- Write denied: cell stays display.
- Server 422 on a value: field error shown on the cell, nothing applied.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- EntityDetail.vue is 4000 lines: put new logic in new components and
composables, keep EntityDetail changes to wiring.
- Table `_props` cost: batch per page; budget test per collection-reads rule.
- The `SectionEditForm` extraction could break list/cards editing: keep its
tests green and run its e2e specs.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes and new section keys

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-LY9SC4O, RR-CQ6QJJX, RR-6YIJRH1, RR-PABTYTY,
RR-3LDEAK1, RR-H7OWQ3H, RR-7D0Q2T7, RR-NFQ2D2G, RR-CJ7ODEZ, RR-VIIGYMT moved
with the split; RR-8I32H8V, RR-8669P8G, RR-3XLCZ00, RR-F35K7DL, RR-PIG9KY8,
RR-JXTTP5L, RR-PA3KBWR handled in this plan; RR-CPB5PXW done by the split.

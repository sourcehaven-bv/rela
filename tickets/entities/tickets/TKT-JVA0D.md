---
id: TKT-JVA0D
type: ticket
title: Fix enum list property input, rendering, and validation in data-entry
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

Properties declared as enum + `list: true` in the metamodel (e.g. `ticket.tags`
→ values: ticket_tag[]) are mishandled across the data-entry UI:

1. **Form validation** (`DynamicForm.vue:286`) treats every property value as a scalar. For list enums, `values.includes(String([...]))` stringifies the whole array and always reports "Must be one of: …", even when every selected option is valid.
2. **Detail / list rendering** (`PropertyDisplay.vue`, `EntityList.vue`, `SidePanel.vue`) renders array values by stringifying to "a, b". They either fall through to `formatValue()` → `value.join(', ')` (plain text), or pass `String(array)` to a single `<Badge>`, which disables the per-value badge styling users expect for enums.
3. **Input widget** (`FieldRenderer.vue:149-164`) renders list enums as the browser default `<select multiple>`, which is clunky — no search, ctrl-click to select, visually out of place next to other form fields. A nice SlimSelect (already used in `SettingsView` via `components/ui/TagSelect.vue`) or styled checkbox group would match the rest of the UI.

## Fix

- **Validation**: validate each array item against `propDef.values` when `propDef.list` is true (already done in working copy at `DynamicForm.vue:286-293`).
- **Rendering**: render enum list values as a row of `<Badge>` components, one per item, in detail, list cells, and side panel — not a single joined-string badge.
- **Input**: replace the multi-`<select>` branch in `FieldRenderer.vue` with the existing `TagSelect` component (SlimSelect-based, searchable, theme-aware).

## Why it matters

`ticket_tag` can't be saved through the form without a spurious error, tags are
visually indistinguishable from free text on detail/list views (losing the
colour cues users rely on), and the multi-select widget is the single clunkiest
input in the data-entry UI — fixing all three in one pass keeps the list-enum
feature end-to-end consistent.

## Acceptance criteria

1. Creating/editing a ticket with one or more `tags` values passes client-side validation and saves.
2. Supplying an invalid value in a list enum still shows "Must be one of …".
3. Scalar enum validation is unchanged.
4. Detail view for a ticket with `tags: [bug, ui]` renders two badges with their per-value colours, not the string "bug, ui".
5. List view cells for a list-enum column render one badge per item (wrapping if tight).
6. Side panel entity references for list-enum-typed properties show per-item badges.
7. Empty / missing list-enum values render as today (no badges, dash or blank).
8. The form input for a list enum is a searchable SlimSelect (or styled checkbox group) — no browser-default `<select multiple>`. Theme-aware, keyboard-accessible, supports deselect.

---
id: PLAN-9ZPK0
type: planning-checklist
title: 'Planning: Fix enum list property validation in data-entry DynamicForm'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- `internal/metamodel`-exposed `list: true` enum properties only (not relations with `properties.*.list`).
- Three user-visible problems: form validation, detail/list/sidepanel rendering, form input widget.
- Frontend only — backend already accepts array values correctly.

OUT of scope:
- Relation cardinality / multi-select of entity references (that's `RelationPicker.vue`).
- Any other field type (single-value enums, booleans, dates).
- New metamodel schema for list types — `list: true` already exists and works.
- Backend property validation changes.

**Acceptance Criteria:**

1. AC1 — Valid list enum submits: Create a ticket with `tags = [bug, ui]`, form validates and POST succeeds. Test: Playwright form spec.
2. AC2 — Invalid list enum rejected: Inject `tags = [bug, bogus]` into `formData`, `validate()` produces "Must be one of …". Test: Vitest unit on validation helper (or component test).
3. AC3 — Scalar enum unchanged: Create/edit a ticket with `status=ready`, invalid `status=foo` still rejected. Test: existing form Playwright spec continues to pass; add/verify scalar regression case.
4. AC4 — Detail view badges: Navigate to a ticket with `tags: [bug, ui]`, DOM contains two `<span class="badge badge--…">` per item. Test: Vitest mount of `PropertyDisplay` with a list-enum property item.
5. AC5 — List cell badges: Configure a list column for `tags`, row cell renders N badges in a flex row (wraps). Test: Vitest mount of `EntityList` cell rendering.
6. AC6 — Side panel badges: Open a side panel where a list-enum property is shown, per-item badges rendered. Test: Vitest mount of `SidePanel`.
7. AC7 — Empty list: `tags: []` or missing renders blank / dash (matches scalar-empty behavior). Test: unit.
8. AC8 — SlimSelect widget: `<FieldRenderer>` for a list-enum property renders `TagSelect` (SlimSelect under the hood), not a `<select multiple>`. Search input works; selecting options updates `modelValue`. Test: Vitest mount of `FieldRenderer` with list propDef + Playwright interaction test.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `slim-select` v3.4.3 is already a direct dependency (`frontend/package.json`) — no new library needed.
- `frontend/src/components/ui/TagSelect.vue` already wraps `SlimSelect` with theme variables, search, closeOnSelect=false, allowDeselect. Used in `views/SettingsView.vue:619` for palette badge config. Ready to reuse.
- `Badge.vue` (`components/common/Badge.vue`) already handles per-value styling lookup through `schemaStore.styles`. The data is there; only the wrappers need to render N of them instead of one joined string.
- FEAT-020 "Multi-select widget for list properties" is the umbrella feature being extended — this ticket completes its end-to-end story.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Validation** — already in working copy. Keep `DynamicForm.vue:286-293` as-is: pick items = array or [value]; `.some()` against allowed values.
2. **Rendering** — add an "array-of-badge" branch wherever a single `<Badge>` is rendered for an enum value:
   - `PropertyDisplay.vue` `shouldUseBadge()` → if value is array and enum, render a `<div class="badge-row">` with N `<Badge>` children. Use existing `Badge` props per item.
   - `EntityList.vue` enum-column cells — same pattern in mobile + desktop branches.
   - `SidePanel.vue` — audit and apply where an entity-property (non-relation) renders a badge.
   - Add one small helper, e.g. `asArray(value): string[]` in `utils/format.ts`, to avoid re-implementing the `Array.isArray ? value : [value]` ternary in every template.
   - CSS: single `.badge-row { display: inline-flex; gap: 4px; flex-wrap: wrap; }` added once in `Badge.vue`'s export or a shared stylesheet.
3. **Input widget** — in `FieldRenderer.vue`, replace the `<select multiple>` block (lines 149-164) with `<TagSelect :model-value="arrayValue" :options="options" @update:model-value="emit('update', $event)" />`. Remove `handleMultiSelect` since `TagSelect` already emits `string[]`.

**Alternatives considered:**

- Styled checkbox group instead of SlimSelect — rejected because SlimSelect scales better for long tag lists (search), and we already have the component.
- Changing `formatValue()` in `utils/format.ts` to return HTML — rejected: that utility is also used by non-enum callers and returning HTML from a plain formatter invites XSS.
- Keeping the single-`<Badge>` fallback "for now" — rejected: produces visually wrong output on existing data (the tickets metamodel already has `tags: ticket_tag[]`).

**Files to modify:**

- `frontend/src/components/forms/FieldRenderer.vue` — swap multi-select `<select>` for `<TagSelect>`; drop `handleMultiSelect`.
- `frontend/src/components/common/PropertyDisplay.vue` — handle array values in the enum branch; render per-item badges.
- `frontend/src/components/lists/EntityList.vue` — same per-item badge rendering in mobile + desktop cells when `isEnumColumn(column)` and value is array.
- `frontend/src/components/forms/SidePanel.vue` — audit property-badge rendering for array values.
- `frontend/src/components/common/Badge.vue` (or a new sibling) — add `.badge-row` wrapper styles once, reused everywhere.
- `frontend/src/utils/format.ts` — add `asArray(value)` helper; possibly a `formatEnumArray()` no-op typing helper.
- `frontend/src/components/forms/DynamicForm.vue` — already done (lines 286-293), keep.
- Tests alongside each: `PropertyDisplay.test.ts`, `EntityList.test.ts`, `FieldRenderer.test.ts` (new if absent), and a Playwright spec for `ticket.tags` round-trip.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Tag values typed into the form → validated client-side against `propDef.values` (strict allowlist); backend re-validates on save.
- Values rendered in badges come from entity JSON (server) and the metamodel (static YAML). Already trusted, but still pass through Vue's text interpolation (no `v-html`). No regression.

**Security-Sensitive Operations:** None. Pure UI; no new fetches, no new RPC
surfaces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see AC1-8 above, each with concrete mount/spec target.

**Edge Cases:**

- Empty array `[]` — renders blank, no badge wrapper.
- Single-item array `[x]` — renders one badge (same as scalar).
- Array containing `null` / `""` — coerce to string via `String(v)`; don't render empty badge.
- Unicode tag values — pass through (Vue escapes).
- A tag value not in `values` (metamodel drift on existing data) — still renders as a gray badge (badge class lookup falls back); list still displays.
- Very long tag values — rely on existing badge styling; verify wrapping works on narrow cells.
- `propDef.list` true but `values` empty — don't render badge row; `TagSelect` shows empty options list (same as scalar enum with empty values).

**Negative Tests:**

- Submitting invalid tag → validation error, form does not POST.
- `<select multiple>` no longer present in DOM (regression guard).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- SlimSelect DOM replacement can interfere with Playwright selectors that target the native select. Mitigation: update affected e2e selectors; SlimSelect exposes stable class names.
- `TagSelect` re-renders on every `options` identity change (memoize with `computed`). Confirm no perf regression on forms with many enums.
- Badge styling: long tag labels may overflow cells. Mitigation: `flex-wrap: wrap` + `overflow: hidden` on the row, verified in responsive test.

Effort: **s** (~1 day: 3 small component changes + a new helper + tests).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — visible behaviour change but no doc currently describes the `list: true` enum widget or rendering. If a guide mentions multi-select, update when implementation is done; otherwise no action needed.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- To be filled after /design-review run -->

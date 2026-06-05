---
id: TKT-UD7YR
type: ticket
title: Route view-side per-field rendering through widget registry
kind: enhancement
priority: high
effort: m
status: done
---

## Goal

Make the `properties`, `cards`, and `list` view display modes in `EntityDetail.vue` render per-field cells via the widget registry shipped in TKT-MZSIJ. Widgets gain a `display` mode; forms continue to use edit mode. The win is cohesion: per-widget rendering logic lives in the widget files, not inline in `EntityDetail.vue`.

This is the structural step that proves the registry is genuinely cross-screen — not form-only — and unlocks TKT-IHCY7 (inline-edit) to flip a widget's mode without re-resolving.

## Scope

### Registry / widget contract

- Add `mode: 'display' | 'edit'` to `WidgetProps` as a **required** prop (no default). Strict typed union. `'inline-edit'` is *not* added — TKT-IHCY7 will widen the union when it implements that mode.
- Add `propertyName?: string` to `WidgetProps` — the wire-level property binding (separate from `propertyDef`, which is the schema entry). SelectWidget passes this to `Badge`'s `property` prop in display mode so badge styling survives view-config property aliases.
- Each of the 8 property widgets grows a `v-if="mode === 'display'"` branch:
  - `TextWidget` / `TextareaWidget` / `NumberWidget`: plain `<span>` with the value
  - `DateWidget`: `<span>` via the existing `formatDate` helper from `frontend/src/utils/format.ts`
  - `CheckboxWidget`: real `<input type="checkbox" disabled aria-readonly>` (RR-UD2I — emoji glyph was changed during review for accessibility)
  - `SelectWidget`: `<Badge>` for a single value, using `propertyName` for style lookup
  - `MultiSelectWidget`: row of `<Badge>` (handles its own array — see "Widgets own their multiplicity" below)
  - `RruleWidget`: `<span>` via the existing `formatValue(value, 'rrule')` helper from `frontend/src/utils/format.ts`

### View-side delegation

- `EntityDetail.vue` `properties` block: `PropertyDisplay.vue` per-value renders via the registry. The DL+chrome layout stays in `PropertyDisplay`; the inline Badge/plain-text logic is removed.
- `EntityDetail.vue` `cards` block (lines 618-654): per-value renders via the registry directly. Cards keep their own `.card-field` chrome.
- `EntityDetail.vue` `list` block (lines 656-679): per-value renders via the registry directly. List keeps its own `.list-fields` chrome.
- Cards and list inline the registry call (`<component :is="resolved" ...>`) rather than extracting a helper — three call sites is below the "extract a helper" threshold.

### Per-section PropertyDef lookup

- Each display-mode-rendering section computes a `Map<fieldKey, PropertyDef>` **once** (Vue `computed`), then passes the resolved `PropertyDef` to each cell. One schema lookup per (section × field), not per (section × entity × field).
- For `cards` sections that mix entity types, key the Map on `(entityType, property)` — still bounded by section config, not by entity count.
- Avoids registering per-cell reactive subscriptions on `schemaStore.entityTypes` (which would re-render the whole view on every schema reload).

### Inaccessible affordance

- New `frontend/src/components/common/InaccessibleField.vue` — owns the lock icon + tooltip + `inaccessibleReason` branching (including the git-crypt special case currently in `PropertyDisplay`).
- All three display modes use the same `v-if="field.inaccessible"` short-circuit pattern around the widget delegation. Single owner, three one-line consumers.

### `FieldRenderer.vue` (form side)

- Updated to pass `:mode="'edit'"` explicitly on the resolved widget. The form side does not depend on a prop default — there isn't one.

## Non-goals

- **No inline-edit on views** — TKT-IHCY7.
- **`table` and `content` display modes deferred.** `table` has more structure (grouping, link columns, per-cell `widget` hint that's already in the wire shape but unused); `content` is markdown. Both ship later. The current `Badge`-and-plain-text logic stays for them.
- **No config changes** — `widget` and `editable` on `ViewSectionField` are TKT-HOIX1. Today's wire shape (`property`, `label`, `values`, `propType`, `inaccessible`) is sufficient.
- **No new widget types.**
- **`cards` widget is still excluded from the registry.** The view-side `display: cards` mode (entities-as-cards) is different from the form-side `cards` *relation widget*. The view-side mode is in scope; the relation widget stays out (RR-KT27X from TKT-MZSIJ).
- **No backend changes.** Wire shape unchanged.
- **No `_fields` affordance gating on view side.** Forms gate per-field readonly via `_fields`; views are display-only in this ticket.

## Why this design

### `mode` as a widget prop (not a registry parameter)

A `select` widget rendered in display mode is *still semantically a select* — the same `propertyDef.values` + value-to-label mapping logic is shared between display (Badge for the value) and edit (`<select>` with all options). Splitting into separate `SelectDisplayWidget` and `SelectEditWidget` components duplicates that logic. One widget with a mode branch keeps the widget as the unit of "this is how this type of value renders, in any mode."

Cost: each widget gets a `v-if mode === 'display'` branch. Acceptable for 8 widgets.

### Widgets own their multiplicity

A `MultiSelectWidget` receives the full array on both edit (today: TagSelect with array) and display (new: row of Badges). The widget decides how to render its own value shape. The display-mode caller does **not** loop over `field.values`.

This is a reversal of an earlier "scalar widgets, display mode loops" decision. Reason: the loop-in-caller version forced every consumer (PropertyDisplay, cards, list, eventually TKT-HOIX1's config-driven views) to know about multi-vs-single. Pushing the loop into `MultiSelectWidget` means consumers all do the same thing — call the resolved widget once — and TKT-IHCY7 can flip `mode` without re-resolving.

### `mode` is required, no default

Required prop with a strict typed union means:
- TypeScript catches `'inline-edit'` (and any other typo) at compile time.
- Form-side intent is visible in code: `FieldRenderer` passes `:mode="'edit'"` explicitly.
- TKT-IHCY7's `'inline-edit'` widening will force every widget consumer to be re-examined — which is what we want.

Cost: ~15 minutes of mechanical test updates to add `mode: 'edit'` to existing widget test mounts.

### `Badge` style lookup needs the field's wire-level binding

`Badge.vue` looks up `schemaStore.styles[property][value]`. Today there's an inconsistency: cards/list pass `field.propType`, `PropertyDisplay` passes `propType || field.name`. After this refactor, widgets receive a `propertyDef` (schema entry, with a metamodel name) — but `Badge` needs the *field's* wire-level binding to resolve correctly for views that alias a property.

The fix: a new `propertyName?: string` prop on `WidgetProps`. The display-mode caller passes `field.propType`; `SelectWidget` hands it to `Badge`. Falls back to `propertyDef.name` if absent.

### Section-level PropertyDef lookup, not per-cell

Per-cell `schemaStore.getPropertyDef()` calls inside a `v-for` register reactive subscriptions on the schema for every cell. With cards holding hundreds of entities × multiple fields = thousands of subscriptions. SSE schema reload would invalidate them all.

Section-level computed `Map` resolves once, passes the resolved `PropertyDef` down. One subscription per section, not per cell.

### `PropertyDisplay` survives as the DL+chrome shell

`PropertyDisplay.vue` keeps owning the DL layout. Its content shrinks: per-value rendering delegates to the registry; lock affordance delegates to `<InaccessibleField>`. Cards and list don't go through `PropertyDisplay` — their chrome (`.card-field` row, `.list-fields` flat span) is intentionally different.

### Date and rrule formatting use existing helpers

`frontend/src/utils/format.ts` already exports both `formatDate` and `formatValue(value, 'rrule')`. No new helpers in this ticket. (Cleaning up the duplicate rrule logic in `RruleBuilder.vue` is a separate ticket.)

## Known behaviour deltas

These are intentional, visible diffs introduced by this refactor. Each is acknowledged here so the pre/post diff (see Verification gate) can confirm they are the *only* deltas.

| # | Display mode | Before | After | Justification |
|---|---|---|---|---|
| 1 | cards | Date fields render raw ISO string (`"2026-06-04"`) | Date fields render formatted (`"Jun 4, 2026"` via existing `formatDate`) | Existing `properties` mode already formats dates; cards inheriting the same behaviour is the consistent answer. |
| 2 | list | Same as #1 | Same as #1 | Same. |
| 3 | cards | Rrule fields render raw rrule string (`"FREQ=WEEKLY;BYDAY=MO"`) | Rrule fields render human-readable summary (`"every week on Monday"` via existing `formatValue`) | Same consistency argument. |
| 4 | list | Same as #3 | Same as #3 | Same. |
| 5 | cards | Inaccessible fields render empty | Inaccessible fields render the lock affordance | Consistent with `properties` mode. Users get an explanation for missing values. |
| 6 | list | Same as #5 | Same as #5 | Same. |
| 7 | properties | `shouldUseBadge` predicate is `propType OR isEnumProperty(prop)` | Badge rendering happens whenever `propertyDef.values?.length > 0` | Slightly different predicate; in practice should produce the same outcome for every real field. Audit during implementation: if any field is currently NOT badged but would be after this change (or vice versa), flag it. |
| 8 | all (boolean fields) | Booleans render as `formatValue(true, 'boolean')` text or inline-checkbox markdown toggling | CheckboxWidget display mode renders a real `<input type="checkbox" disabled aria-readonly>` (RR-UD2I) | Native semantics for screen readers; consistent rendering across OSes (vs glyph fallback hell); same DOM shape as edit mode minus the interactivity. |
| 9 | cards/list (empty multi-select) | Multi-value fields with empty values rendered nothing | MultiSelectWidget renders an em-dash ("—") placeholder (RR-UD2C) | Distinguishes "no value" from "loading" or "field missing". |
| 10 | cards/list (long arrays) | All values rendered as Badges regardless of count | Arrays with >5 values fall back to a comma-joined string (RR-UD2C) | Prevents 50+ Badges from breaking card layouts; preserves readability. Threshold hardcoded; revisit if a config knob is needed. |
| 11 | all (Badge styling) | Badge fell back to a cross-property scan when `:property=` was absent | Badge returns the gray fallback when `:property=` is absent or unmatched (RR-UD2D) | Cross-property scan was non-deterministic (depended on JS Object iteration order). Audit confirmed no production code relied on it; only tests had to be updated. |

Any diff not on this list is a regression and must be fixed before merge.

## Verification gate

The "no behaviour change beyond the listed deltas" claim is verified by:

1. **Per-widget display-mode unit test.** Each of the 8 widgets gets a test asserting the rendered DOM shape for `mode='display'` given a documented input. Covers: routing (correct widget for correct propertyDef), structural shape (correct element, correct class), null/undefined handling, type mismatch fallback. ~3 cases × 8 widgets ≈ 24 assertions. This catches forward regressions in widget routing and DOM structure.

2. **Pre/post DOM diff at merge.** One-time gate, not a CI test. Before merge: capture rendered HTML of populated `properties`/`cards`/`list` sections on a representative entity, both on `develop` and on this branch. Diff them. Confirm the diff matches the Known behaviour deltas section above and nothing else. Document the diff in the review checklist as evidence.

3. **Visual browser smoke** on a populated entity detail in the local dev server (against the `tickets/` project). Screenshot before, screenshot after. Compare. Same purpose as the DOM diff but catches CSS regressions the snapshot can't.

4. **`max-lines: 500` ESLint check on `EntityDetail.vue`.** Current size is ~800 lines; the refactor should bring it closer to (or under) 500. Not a hard pass/fail, but track the delta in the review checklist.

5. **Existing tests pass unchanged.** `FieldRenderer.test.ts`, `widgets.test.ts`, `wrapperWidgets.test.ts`, `FieldShell.test.ts` — all updated to pass `mode: 'edit'` explicitly, no other changes. Confirms backward compatibility for the form side.

## Design-review findings addressed

All 12 findings (RR-UD1A through RR-UD1L) are linked via `has-review-response`. Critical: RR-UD1A (formatDate reuse), RR-UD1B (rrule reuse), RR-UD1C (known deltas section). Significant: RR-UD1D (test strategy), RR-UD1E (propertyName prop), RR-UD1F (InaccessibleField extraction), RR-UD1G (widgets own multiplicity), RR-UD1H (section-level PropertyDef lookup). Minor: RR-UD1I (no default, required prop), RR-UD1J (LoC claim dropped), RR-UD1K (strict union). Nit: RR-UD1L (PropertyDisplay survives).

## Out of scope (deferred)

- `useInlineEdit` composable and `'inline-edit'` mode widening — TKT-IHCY7
- View-config `widget` / `editable` fields — TKT-HOIX1
- Markdown body inline-edit — TKT-GUPMK
- `table` and `content` display mode delegation
- `cards` *relation* widget under the registry — separate `RelationWidgetRegistry` if/when needed
- RruleBuilder.vue duplicate rrule logic cleanup — separate refactor
- The `shouldUseBadge` predicate audit (Known delta #7) — if it surfaces a real diff during implementation, file a follow-up bug

<!-- @managed: claude-workflow v1 -->
---
id: PLAN-UD7YR
type: planning-checklist
title: 'Planning: Route view-side per-field rendering through widget registry'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-UD7YR ticket body, "Scope" section.

**Acceptance Criteria:**

1. `WidgetProps` gains a **required** `mode: 'display' | 'edit'` prop with a strict typed union. No default. Existing widget tests are updated to pass `mode: 'edit'` explicitly; all still pass.
2. `WidgetProps` gains an optional `propertyName?: string` prop carrying the field's wire-level binding (separate from `propertyDef`). SelectWidget passes it to Badge in display mode.
3. Each of the 8 property widgets renders correctly in display mode: text/textarea/number → span, date → formatted span via existing `formatDate`, checkbox → ✓/☐ glyph, select → Badge, multi-select → row of Badges (widget owns its multiplicity), rrule → human summary via existing `formatValue(value, 'rrule')`.
4. `PropertyDisplay.vue` renders each value via the registry in display mode (instead of inline Badge/plain text). DL+chrome layout stays.
5. `EntityDetail.vue` `cards` block renders each value via the registry directly. `.card-field` chrome stays.
6. `EntityDetail.vue` `list` block renders each value via the registry directly. `.list-fields` chrome stays.
7. New `InaccessibleField.vue` component owns the lock affordance + reason tooltip + git-crypt special case. All three display modes use it.
8. Each display-mode-rendering section computes a `Map<fieldKey, PropertyDef>` once per section; cells receive the resolved def directly. No per-cell `schemaStore.getPropertyDef()` calls.
9. `table` and `content` display modes are unchanged.
10. `FieldRenderer.vue` passes `:mode="'edit'"` explicitly. The default-less prop forces all consumers to be explicit.
11. Per-widget display-mode unit tests cover routing, structural shape, null/undefined handling, type mismatch fallback. ~24 assertions.
12. Pre/post DOM diff at merge captures the rendered HTML of populated `properties`/`cards`/`list` sections on `develop` vs this branch. Diff matches the Known behaviour deltas in the ticket and nothing else.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: extension of TKT-MZSIJ registry)
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- No external library — the registry already returns a Vue component; widgets just need a `mode` branch.
- `frontend/src/utils/format.ts` already exports `formatDate` and `formatValue` (including rrule). Reuse, do not extract from RruleBuilder.
- Prior art in this codebase: TKT-MZSIJ shipped the registry (#848). `FieldRenderer.vue` is the consumer pattern on the form side.
- `RruleBuilder.vue` has a duplicate rrule preview helper. Cleanup is a separate refactor — out of scope here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:** see TKT-UD7YR ticket body for the full rationale. Implementation steps:

1. **Widget contract update**:
   - `frontend/src/widgets/types.ts`: add required `mode: 'display' | 'edit'`, optional `propertyName?: string`.
2. **8 widget edits**: each gains a `v-if="mode === 'display'"` branch. Display branches reuse `formatDate` / `formatValue` from `utils/format.ts`. `SelectWidget` display branch renders `<Badge :value="modelValue" :property="propertyName ?? propertyDef?.name">`. `MultiSelectWidget` display branch loops internally over the array.
3. **`InaccessibleField.vue`** (new) under `frontend/src/components/common/`. Owns the lock icon, reason-aware tooltip, git-crypt special case. Migrate the affordance out of `PropertyDisplay.vue`.
4. **`PropertyDisplay.vue` slim-down**: per-value rendering delegates to the registry; lock affordance delegates to `<InaccessibleField>`.
5. **`EntityDetail.vue` cards block**: replace inline Badge/span with `<InaccessibleField v-if ... />` + registry-resolved widget. Same shape for list block.
6. **Per-section PropertyDef Map**: computed up-top in each section template (or pre-computed in script for cards/list). Each cell receives the resolved `PropertyDef` directly. Avoid `schemaStore.getPropertyDef()` inside the loop.
7. **`FieldRenderer.vue`**: pass `:mode="'edit'"` explicitly.
8. **Existing widget tests**: search-and-replace add `mode: 'edit'` to each mount call. Mechanical.
9. **New per-widget display-mode tests**: ~3 per widget, ~24 total assertions.
10. **Pre/post DOM diff at merge**: one-time gate, not a CI test. Captured in review checklist.

**Alternatives considered:**

- *Mode-aware registry resolver (multi-select collapses to select in display mode).* Rejected — RR-UD1G. Pushes mode awareness into the registry, complicates TKT-IHCY7's mode-flipping. Better to let `MultiSelectWidget` own its display branch.
- *Scalar widgets, display mode loops.* Rejected — RR-UD1G. Forces every consumer to know about multi-vs-single; widget should own its shape.
- *Default `mode` to `'edit'`.* Rejected — RR-UD1I. Defaults are silently load-bearing; required prop catches mistakes at compile time.
- *Inline-edit reserved in the union now.* Rejected — RR-UD1K. Soft commitment to an unimplemented value; TKT-IHCY7 widens the union when it lands.
- *Extract rrule helper from RruleBuilder.* Rejected — RR-UD1B. `utils/format.ts` already has the helper; extracting from RruleBuilder is a third copy.
- *Per-cell `schemaStore.getPropertyDef()`.* Rejected — RR-UD1H. Reactive cost on schema reload. Section-level computed Map is the answer.
- *PropertyDisplay folded into delegation.* Rejected — RR-UD1L. DL+chrome is shared across many sections; cards/list have different chrome and intentionally don't go through it.
- *Composable for the registry-call + lock-check pattern across cards/list/PropertyDisplay.* Rejected — RR-UD1L. Three call sites is below the "extract a helper" threshold.

**Files to modify:**

- `frontend/src/widgets/types.ts` (required `mode`, optional `propertyName`)
- `frontend/src/widgets/{Text,Textarea,Number,Checkbox,Date,Select,MultiSelect,Rrule}Widget.vue` (display branches; `MultiSelectWidget` owns its loop; `SelectWidget` consumes `propertyName`)
- `frontend/src/components/common/InaccessibleField.vue` (new)
- `frontend/src/components/common/PropertyDisplay.vue` (delegate per-value to registry; delegate lock to InaccessibleField)
- `frontend/src/components/entity/EntityDetail.vue` (cards + list delegations; per-section PropertyDef Map)
- `frontend/src/components/forms/FieldRenderer.vue` (pass `:mode="'edit'"`)
- `frontend/src/widgets/widgets.test.ts` (mode prop added to all mounts; new display-mode tests)
- `frontend/src/widgets/wrapperWidgets.test.ts` (mode prop added)
- `frontend/src/components/forms/FieldShell.test.ts` and `FieldRenderer.test.ts` (mode prop added where mounts go through)
- `frontend/src/components/common/InaccessibleField.test.ts` (new)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Field values from the view API*: source is `internal/dataentry/api_v1.go`. Already type-safe; display widgets stringify/format. No HTML injection — widgets use Vue text interpolation, never `v-html`. Date and rrule values pass through existing `formatDate` / `formatValue` helpers which already handle malformed input by falling back to the raw string.
- *Inaccessible flag*: `field.inaccessible === true` short-circuits the widget call (via `<InaccessibleField>`) before any widget is invoked. The widget never sees the (absent) value. Preserves the TKT-G7N5 inaccessibility contract.
- *`propertyName` prop*: passed straight through to `Badge`'s schema-styles lookup. Already-validated wire value; no new attack surface.

**Security-Sensitive Operations:** None new. Internal refactor; no new I/O, no new auth surface.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| Acceptance criterion | Test |
|---|---|
| `mode` required, no default | TypeScript compile error when omitted; type widening would force re-examination. |
| Each widget renders correctly in display mode | `widgets.test.ts` display-mode block per widget, ~3 cases each. |
| `propertyName` flows to Badge | `SelectWidget` display-mode test mounts with a `propertyName` and asserts `Badge.property` equals it. |
| `PropertyDisplay` delegates to registry | Behaviour test: mount with a fixture, assert no inline Badge in the DOM, only the resolved widget. |
| `cards` / `list` delegate to registry | Same. |
| `InaccessibleField` owns the lock | New `InaccessibleField.test.ts`: covers reason branching, generic fallback, git-crypt tooltip. |
| Per-section PropertyDef lookup | Spy on `schemaStore.getPropertyDef`; assert it's called once per (section × field), not per cell. |
| `table` and `content` unchanged | Their existing snapshots/tests pass without modification. |
| Form side unchanged | `FieldRenderer.test.ts` updated to pass `mode: 'edit'`; otherwise passes. |
| Pre/post DOM diff | Manual at merge. Document in review checklist with the actual diff. |

**Edge Cases:**

- `value === null` / `undefined` for every widget → each widget defines its empty render (documented per widget).
- `values: []` (empty array) on a multi-select field → `MultiSelectWidget` display renders nothing (empty row).
- `propType` unset on the `ViewSectionField` → falls back to `TextWidget` in display mode.
- Inaccessible field → `<InaccessibleField>` short-circuit; widget never invoked.
- Date string that fails to parse → existing `formatDate` falls back to the raw string + console.warn.
- Rrule string that fails to parse → existing `formatValue` falls back to the raw string + console.warn.
- Boolean rendered with `'true'`/`'false'` strings (server can return either) → `CheckboxWidget` display mode normalises.
- View-config aliased property (display label differs from metamodel name) → `propertyName` carries the wire binding; Badge style lookup remains correct.
- Schema reload via SSE → section-level `Map` re-computes once; cells re-render with the new defs. No per-cell churn.

**Negative Tests:**

- `mode: 'inline-edit'` → TypeScript error (caught at compile). No runtime negative test needed.
- `mode` omitted → TypeScript error.
- `PropertyDef` lookup fails (orphan field) → widget falls back to `TextWidget` in display mode + console.warn.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated — `m`

**Risks:**

| Risk | Mitigation |
|---|---|
| Visual regression invisible to existing tests (font, spacing, badge styling) | Per-widget display-mode tests + pre/post DOM diff at merge + visual browser smoke. RR-W3J1A pattern from TKT-MZSIJ. |
| Behaviour deltas surface as bugs after merge | Explicit "Known behaviour deltas" table in the ticket. Pre/post diff confirms the diff matches the list and nothing else. |
| Cards/list lock affordance creates user-visible change where there was none | Documented as known delta #5/#6. Justified as a fix (users see why a value is missing). |
| `mode` prop required, breaks every widget test on import | Mechanical search-and-replace, 15 minutes. |
| Section-level Map computed on every reactive read of schemaStore | Vue's `computed` memoizes by dep tracking. One re-compute per real schema change. Confirmed by spy test (assertion: schema lookup called once per (section × field), not per cell). |
| `propertyName` introduction breaks existing Badge behaviour at form-side call sites | FieldRenderer doesn't go through Badge (forms use selects/inputs). View side gets `propertyName` from `field.propType`. Existing FieldRenderer tests pass unchanged. |

## Documentation Planning

- [x] User-facing docs identified — N/A
- [ ] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- N/A — Internal refactor. No user-facing API change, no config format change. The registry contract grew `mode` (required) and `propertyName` (optional) — additive widget-side changes only, with no surface visible to end users. Behaviour deltas are documented in the ticket and confirmed by manual gate at merge.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

All 12 findings addressed in the revised plan:

- **Critical (3)**: RR-UD1A (formatDate reuse), RR-UD1B (rrule reuse), RR-UD1C (known deltas section)
- **Significant (5)**: RR-UD1D (test strategy), RR-UD1E (propertyName prop), RR-UD1F (InaccessibleField), RR-UD1G (widgets own multiplicity), RR-UD1H (section-level PropertyDef lookup)
- **Minor (3)**: RR-UD1I (no default, required), RR-UD1J (LoC claim dropped), RR-UD1K (strict union)
- **Nit (1)**: RR-UD1L (PropertyDisplay survives)

Resolution details and rationale live in the TKT-UD7YR ticket body and in each `RR-UD1x` review-response.

---
id: PLAN-1AA4M
type: planning-checklist
title: 'Planning: Extract shared widget registry from FieldRenderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-MZSIJ ticket body, "Scope (revised after design-review)".

**Acceptance Criteria:**

1. `defaultRegistry` exposes a `resolve(name, propertyDef)` that returns the correct widget component for every (propertyType, widget, list, values) combination present in the repo's data-entry configs today — verified by snapshot test.
2. `FieldRenderer.vue` no longer contains a per-widget `v-if`; it delegates to the registry + `FieldShell`.
3. `DynamicForm.vue` is unmodified (form code does not need to know the registry exists).
4. Each of the 8 in-scope widgets (text, textarea, checkbox, select, multi-select, date, number, rrule) renders identically (DOM structural equality) before/after the refactor.
5. Tests can construct an isolated `defineWidgetRegistry()` and register stubs without module mocking.
6. `cards` continues to work via its existing path; no behaviour change to RelationCards.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- No external library — `FieldRenderer.vue` already does the dispatch; the work is structural, not algorithmic.
- The factory-registry pattern is used in this codebase for consumer-side interfaces (see CLAUDE.md "Consumer-side interfaces" section). Vue-side analogue: any composable that returns `{register, resolve}`. No prior frontend instance.
- `mcp.Services` (`internal/mcp/server.go`) and `scheduler.WorkspaceProvider` (`internal/scheduler/scheduler.go`) are the architectural prior art: narrow consumer-side interfaces supplied at construction. The widget registry mirrors this — callers receive `WidgetRegistry`, not the concrete map.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** see TKT-MZSIJ ticket body, "Revised contract (post
design-review)" section.

Summary:

1. Extract per-widget render logic from `FieldRenderer.vue` template into 8 single-purpose Vue components (`TextWidget.vue`, `CheckboxWidget.vue`, …).
2. Each widget uses `defineProps<WidgetProps<T>>()` + `defineEmits<{'update:modelValue': [T]}>()` — standard Vue 3 idiom (RR-G3AD6).
3. Introduce `frontend/src/widgets/registry.ts` with `defineWidgetRegistry()` factory and `defaultRegistry` singleton (RR-944BN).
4. Introduce `frontend/src/components/forms/FieldShell.vue` that owns label/help/error/layout. Widgets render only the input control (RR-IRLQ7).
5. `FieldRenderer.vue` becomes: receive field config → `defaultRegistry.resolve(field.widget, propertyDef)` → render `<FieldShell>` wrapping the resolved component bound via `v-model` and props.
6. `cards` widget stays on its existing path; not part of the property registry (RR-KT27X).
7. Audit `multiselect` vs `multi-select` in repo configs; pick one canonical name (RR-DKS9B).

**Alternatives considered:**

- *Keep `mode` prop now, anticipating TKT-UD7YR.* Rejected — see RR-DGRKQ. Adds a single-value prop into 8 widget signatures with no use site to validate it.
- *Static `Record<string, WidgetEntry>` registry.* Rejected — see RR-944BN. Test ergonomics push toward factory.
- *Widgets own their own label/help/error.* Rejected — see RR-IRLQ7. Eight inconsistent label implementations is a regression.
- *Force `cards` into `WidgetProps<T>`.* Rejected — see RR-KT27X. Relation widgets are a different shape; forcing one contract produces a leaky abstraction.

**Files to modify:**

- `frontend/src/components/forms/FieldRenderer.vue` (gut + delegate)
- `frontend/src/widgets/` (new directory: `registry.ts`, `TextWidget.vue`, `TextareaWidget.vue`, `CheckboxWidget.vue`, `SelectWidget.vue`, `MultiSelectWidget.vue`, `DateWidget.vue`, `NumberWidget.vue`, `RruleWidget.vue`)
- `frontend/src/components/forms/FieldShell.vue` (new)
- `frontend/src/widgets/registry.test.ts` (new)
- One snapshot test fixture per widget (new)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- *Widget name from config (`field.widget`)*: source is `internal/dataentryconfig/`, already validated server-side via `validate.go`. The registry resolver treats unknown widget names by falling through to `defaultWidgetFor(propertyDef)` and logs a `console.warn`. No DOM injection — widget names map to component references, never used as HTML/CSS strings.
- *Property values rendered by widgets*: unchanged from today. Each widget continues to use Vue's default escaping; no `v-html` introduced.
- *Option values for select/multi-select*: passed through unchanged; existing escaping holds.

**Security-Sensitive Operations:** None new. This is a structural refactor; no
new I/O, no new auth surface, no new user-input handling.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| Acceptance criterion | Test |
|---|---|
| Registry resolves correct widget for every config combination | Snapshot test enumerating every (propertyType, widget, list?, values?) tuple in repo configs |
| FieldRenderer delegates to registry | Vitest assertion that FieldRenderer renders the resolved component for a given config |
| DynamicForm unmodified | Existing form e2e tests pass without change |
| DOM equality before/after | Per-widget Vitest snapshot of rendered DOM, compared to baseline captured pre-refactor |
| Test-local registries work | Vitest test constructs `defineWidgetRegistry()`, registers a stub, asserts resolve returns stub |
| `cards` unchanged | Existing RelationCards tests pass |

**Edge Cases:**

- Field config with `widget: undefined` → fall through to default
- Field config with `widget: 'unknown'` → console.warn, fall through to default
- `propertyDef.list === true` + explicit `widget: 'text'` → honour explicit widget (no override)
- `propertyDef.values: []` (empty array) → treat as not enum, fall through type-based defaults
- `modelValue === null` and `modelValue === undefined` → each widget defines a documented empty render; covered per-widget
- Widget receiving `modelValue` of unexpected type (server returned stale schema) → widget renders empty + console.warn; does not throw

**Negative Tests:**

- Resolver called with no propertyDef → throws (programming error, not a config issue)
- Test registers a widget then attempts to register the same name → second register wins, logs a warning (mirrors Vue component re-registration)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — `m`

**Risks:**

| Risk | Mitigation |
|---|---|
| Keystroke-level regression invisible to existing tests (focus, IME, ARIA, keyboard nav) | Per-widget snapshot test + manual smoke on every metamodel entity type + one-week bake before tickets 2-5 start (RR-W3J1A) |
| `(propertyType, widget)` pair currently tolerated breaks | `supportedPropertyTypes` is advisory only — console.warn, never refuse render. Tightening is a follow-up gated on config audit (RR-036SN) |
| Pre-existing `multiselect`/`multi-select` mismatch surfaces during refactor | Audit configs as sub-task; document choice in this checklist before coding (RR-DKS9B) |
| Contract changes after 8 widgets are written against it | Design-review pass already lifted cross-cutting props (RR-ABTFH); future tickets only *add* (mode prop, commit emit), never *remove*. The opt-in `commit` emit is pre-declared so TKT-IHCY7 doesn't need a re-cut |

## Documentation Planning

- [x] User-facing docs identified — N/A
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal refactor, no user-facing surface)

**Documentation Impact:**

- N/A — Internal refactor. No user-facing API change, no config format change, no behaviour change. Widget authors won't exist as a separate audience until plugin-style widgets ship (not in this ticket).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **Critical** (3, all addressed): RR-ABTFH, RR-KT27X, RR-DKS9B
- **Significant** (7, all addressed): RR-IRLQ7, RR-0Z1P6, RR-G3AD6, RR-DGRKQ, RR-944BN, RR-036SN, RR-W3J1A
- **Minor** (2): RR-3DJJF addressed (PropertyType union); RR-RP3HT deferred (T=unknown stays for now; narrowing is a follow-up — accepted as a known limitation)

Resolution detail and decisions live in the TKT-MZSIJ ticket body and the
"Resolution of original open questions" table.

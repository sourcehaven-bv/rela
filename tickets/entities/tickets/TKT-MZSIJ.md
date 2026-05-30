---
id: TKT-MZSIJ
type: ticket
title: Extract shared widget registry from FieldRenderer
kind: enhancement
priority: high
effort: m
status: in-progress
---

## Goal

Pull the widget dispatch logic out of
`frontend/src/components/forms/FieldRenderer.vue` (348 lines, a template `v-if`
switching on widget name) into a standalone widget registry that both forms and
(in later tickets) views can call. No behaviour change for forms.

## Scope (revised after design-review)

- Define a `defaultRegistry: WidgetRegistry` built from the **property** widgets that exist today.
- Each widget is a Vue component using the project's `update:modelValue` + `emit` idiom (not React-style `onChange` prop).
- `FieldRenderer.vue` becomes a thin shell that resolves the widget from the registry and slots it into the existing label/help/error layout.
- The shell (renamed `FieldShell` internally) keeps owning label, help, error, layout — widgets render only their input/control.
- **`cards` is excluded** from the registry (see RR-KT27X). `cards` renders relations, not property values; it gets its own dispatch path. May earn a place in a separate `RelationWidgetRegistry` in a follow-up if needed.
- **No `mode` prop in this ticket** (see RR-DGRKQ). Forms render edit; `mode` is added in TKT-UD7YR when view-side delegation actually needs it.

## Non-goals

- No view-side delegation.
- No new widget types.
- No `_actions` / `_fields` affordance changes.
- No backend changes.

## Why this first

The registry's contract is the load-bearing API the next four tickets depend on.
Worth nailing the prop surface and the Vue idiom before nine widgets are written
against it.

---

## Revised contract (post design-review)

### Widget component shape

Each widget is a Vue 3 SFC using `defineProps` + `defineEmits`. Cross-cutting
concerns are first-class props (RR-ABTFH); per-widget config goes through
`options`.

```ts
// Shared across all property widgets.
interface WidgetProps<T = unknown> {
  // Bound via v-model. Widgets emit 'update:modelValue' on change. (RR-G3AD6)
  modelValue: T

  // Metamodel property definition. Widgets that need to know about
  // enum values, list:true, format hints get them from here. (RR-0Z1P6)
  propertyDef: PropertyDef

  // Cross-cutting state. Lifted out of options (RR-ABTFH).
  disabled?: boolean
  readonly?: boolean
  required?: boolean
  error?: string
  id?: string
  placeholder?: string

  // Per-option ACL verdicts (consumed by select/multi-select). (RR-ABTFH)
  optionVerdicts?: Record<string, boolean>

  // Per-widget extra config (rrule defaults, etc.). Genuinely
  // widget-specific things only — anything cross-cutting is a top-level prop.
  options?: WidgetOptions
}

// Emits (Vue idiom; see RR-G3AD6).
type WidgetEmits<T> = {
  'update:modelValue': [T]   // buffered change; caller decides when to persist
  commit?: [T]               // explicit "persist now" (Enter, blur) — used by future TKT-IHCY7
}
```

`PropertyType` is a discriminated union from the metamodel, not a free string
(RR-3DJJF):

```ts
type PropertyType = 'string' | 'boolean' | 'date' | 'integer' | 'rrule' | 'markdown' | 'enum'
```

### Registry shape (factory, not static map)

`defineWidgetRegistry()` returns an object; a `defaultRegistry` is constructed
from the production widgets. Tests construct their own (RR-944BN).

```ts
interface WidgetEntry {
  component: Component
  // Diagnostic only in this ticket — logs a console.warn on mismatch,
  // does NOT refuse to render (RR-036SN).
  supportedPropertyTypes?: PropertyType[]
}

interface WidgetRegistry {
  register(name: string, entry: WidgetEntry): void
  resolve(name: string | undefined, propertyDef: PropertyDef): Component
}

export function defineWidgetRegistry(): WidgetRegistry
export const defaultRegistry: WidgetRegistry  // built from production widgets
```

### Default widget resolution (preserves today's logic)

`resolveWidget(name, propertyDef)` honours **explicit widget name first**, then
falls back to multi-axis defaulting that matches FieldRenderer today (RR-0Z1P6).
The fallback order is:

1. `propertyDef.list === true` → `multi-select`
2. `propertyDef.values?.length > 0` → `select`
3. `propertyDef.type === 'boolean'` → `checkbox`
4. `propertyDef.type === 'date'` → `date`
5. `propertyDef.type === 'integer'` → `number`
6. `propertyDef.type === 'rrule'` → `rrule`
7. else → `text`

This is encoded in a single `defaultWidgetFor(propertyDef)` function. Any change
to this is a behaviour change and a separate ticket.

### Widget name normalization (RR-DKS9B)

FieldRenderer today checks `widget === 'multiselect'` (no hyphen). Go config
emits `'multi-select'` (with hyphen). **Sub-task in this ticket:** audit which
is actually used in configs in the repo. Normalize on `multi-select` (matches Go
side and `validate.go`). If `multiselect` configs exist, either rewrite them or
accept both via the registry for one release with a deprecation log. Document
choice in the planning checklist.

### Label/help/error ownership (RR-IRLQ7)

A `FieldShell.vue` component renders the `<label>` (with required asterisk),
help paragraph, error paragraph, and `.form-field` layout. It slots the widget
in the input position. Widgets themselves render only the input control. The
checkbox label-position special-case becomes a `labelPosition: 'before' |
'after'` prop on `FieldShell`, not a per-widget concern.

`FieldRenderer.vue` becomes a thin glue: takes a field config, resolves the
widget from the registry, wraps it in `FieldShell`. Form code
(`DynamicForm.vue`) does not need to change.

### Verification gate (RR-W3J1A)

This ticket asserts "no behaviour change for forms." That claim is verified by:

1. **Snapshot test per widget**: for every `(propertyType, widget, optionVerdicts?)` combination currently in any data-entry config in the repo, render the field via FieldRenderer (before) and via the new registry-backed FieldRenderer (after); assert DOM structural equality.
2. **Inventory existing form e2e tests**: cover at minimum text input, select, multi-select, checkbox, date, rrule. Add e2e coverage for any widget without an existing test before refactoring.
3. **Manual smoke test** on every entity type in the repo's `metamodel.yaml` to catch label/help/error layout drift the snapshot doesn't catch.
4. Refactor is reviewable as **"FieldRenderer template → 8 widget components"** in one PR with the original template preserved as comments for one release, deleted in a follow-up commit after a one-week bake.

### Resolution of original open questions

| # | Question | Decision (with finding) |
|---|---|---|
| 1 | `mode` prop? | **Drop it in this ticket** (RR-DGRKQ); add in TKT-UD7YR |
| 2 | `onChange` async vs sync? | **Moot — use Vue `update:modelValue` + `commit` emit** (RR-G3AD6); persistence state flows down as props |
| 3 | `WidgetOptions` typing | Discriminated union per widget name; shape is small |
| 4 | Map vs factory? | **Factory** (RR-944BN); `defaultRegistry` is the production singleton |
| 5 | `defaultWidgetFor` location | Frontend constant for now; metamodel `default_widget` is a follow-up |
| 6 | Backward compat | Names match Go side after `multiselect`/`multi-select` audit (RR-DKS9B); `(propertyType, widget)` pair compatibility verified by snapshot test (RR-036SN) |

### Out of scope (deferred)

- View-side delegation (TKT-UD7YR)
- `useInlineEdit` composable and persistence state props (TKT-IHCY7)
- View-config widget/editable fields (TKT-HOIX1)
- Markdown body inline-edit (TKT-GUPMK)
- `cards` widget under the registry (separate `RelationWidgetRegistry` if/when needed)
- Stricter `(propertyType, widget)` validation that rejects mismatches (advisory only in this ticket; tightening is a follow-up gated on a config audit)

### Design-review findings addressed

All 12 findings are linked via `has-review-response`. Critical: RR-ABTFH
(cross-cutting props), RR-KT27X (cards excluded), RR-DKS9B (name mismatch
audit). Significant: RR-IRLQ7, RR-0Z1P6, RR-G3AD6, RR-DGRKQ, RR-944BN, RR-036SN,
RR-W3J1A. Minor: RR-3DJJF, RR-RP3HT (addressed via `PropertyType` union;
`T=unknown` accepted as a known limitation — narrowing is a follow-up).

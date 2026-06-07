---
id: TKT-IHC7B
type: ticket
title: Properties-section inline edit via SectionEditForm
kind: enhancement
priority: high
effort: m
status: backlog
---

## Goal

Make the properties section of `EntityDetail` (the DL of name/value pairs rendered by `PropertyDisplay`) inline-editable via a new `SectionEditForm` host component. Per-cell writability is gated by `entry._fields[prop]?.writable`.

This is the middle slice of the split TKT-IHCY7 (see status note in TKT-IHCY7). It builds on TKT-IHC7A's `useAutoSave` API extensions (per-channel debounce, initial snapshot, channel disable).

## Scope (high-level — to be refined in planning)

### `SectionEditForm.vue` component

A new component under `frontend/src/components/forms/` that wraps a property set of one entity in a `useAutoSave` instance and **owns the iteration** (resolving RR-UE3C option (c)). Does not use provide/inject; does not require slot consumers to inject anything.

Props (sketch — refine in planning):

```ts
defineProps<{
  entityType: string
  entityId: string
  initialValues: Record<string, unknown>
  // Per-property metadata for iteration: widget hint, writability.
  // The component iterates this list and resolves each widget via the
  // registry.
  fields: Array<{
    property: string
    label: string
    propertyDef?: PropertyDef
    routingHint?: WidgetRoutingHint
    writable: boolean
  }>
  // Host-provided write-back so viewData.entry stays consistent with
  // the section's local formData.
  onPropertyApplied: (prop: string, value: unknown) => void
  // Host-provided error surface (defaults to uiStore.error).
  onError?: (msg: string) => void
}>()
```

The component:
- Owns the iteration over `fields`. For each writable field, renders the resolved widget in `'inline-edit'` mode bound to `formData[field.property]`. For non-writable fields, renders the widget in `'display'` mode.
- Instantiates `useAutoSave` for this section's property set, using `initialServerSnapshot: initialValues` (from TKT-IHC7A's API extension) for atomic baseline seeding.
- On `update:modelValue` from a writable widget, calls `useAutoSave.scheduleFieldSave(property, value)`. **No provide/inject** — the iteration is internal so the wiring is direct.
- On `applyServerProperty(prop, value)` from `useAutoSave`, updates local `formData` AND calls `onPropertyApplied(prop, value)` so the host can write back to `viewData.entry`.
- Renders one `AutoSaveIndicator` inline-right of the section heading (host-provided slot for placement).
- Calls `commitImmediately` on unmount (matches `DynamicForm` today).

### Widget contract change

Widen `WidgetMode` to `'display' | 'edit' | 'inline-edit'` in `frontend/src/widgets/types.ts`. The `'inline-edit'` render is *typically identical to `'edit'`* — widgets only differ where chrome would be visually inappropriate inline (e.g., `SelectWidget` drops its transitions-info panel; the FieldShell wrapper is absent because `SectionEditForm` doesn't wrap the widget in `FieldShell`).

Per-widget mode-extension tests assert the `'inline-edit'` render shape AND that `update:modelValue` round-trips correctly for the widget's value type (resolving RR-UE3I).

### EntityDetail integration

Replace `PropertyDisplay`'s call site for sections that have at least one writable property with `<SectionEditForm>` — passing the resolved widget rendering decisions through the `fields` prop. Sections with all-non-writable properties continue to use `PropertyDisplay` (no change).

The host (`EntityDetail`) reads `entry._fields[prop]?.writable` to compute the `writable` flag per field. The host wires `onPropertyApplied` to write back to `viewData.entry._props[prop]`. The host wires `onError` with 401/403 special-case ("permission changed — refresh to see current affordances").

## Non-goals

- **No cards/list inline edit.** TKT-IHC7C.
- **No view-config surface.** TKT-HOIX1.
- **No optimistic UI.**
- **No new wire-shape additions for `_fields`** — it's already on the wire.

## Why this design

### `SectionEditForm` owns the iteration (RR-UE3C resolution)

Three options were on the table for properties-section integration:
- (a) `PropertyDisplay` grows a `mode` prop. Rejected — drags `PropertyDisplay` into the autosave domain; outside the ticket's natural scope.
- (b) `EntityDetail` writes a parallel block when writable. Rejected — code duplication.
- (c) `SectionEditForm` owns the iteration via the registry. **Picked.**

Option (c) means `SectionEditForm` is a "small `DynamicForm` for a section" — it knows its fields, resolves widgets, binds to `formData`. The widget contract change is minimal (just the `'inline-edit'` mode addition); the iteration logic lives in one place.

### No provide/inject (RR-UE3A resolution)

Round 3's reviewer correctly pointed out that `provide/inject` doesn't compose: `inject()` runs in the component that *contains* the inject call, not in a parent template, so a slot author can't `inject` the `scheduleFieldSave` function provided by `SectionEditForm`. The fix: `SectionEditForm` doesn't expose a slot — it owns the iteration. Widgets emit `update:modelValue` to `SectionEditForm` directly via the v-for binding in its template, and `SectionEditForm` calls `scheduleFieldSave` internally.

### `initialServerSnapshot` from TKT-IHC7A (RR-UE3F resolution)

TKT-IHC7A adds `initialServerSnapshot` to `useAutoSave`'s constructor. `SectionEditForm` uses it: instead of calling `recordServerSnapshot` after construction (which races with widgets emitting on mount), `SectionEditForm` passes `initialServerSnapshot: initialValues` directly into the constructor. Atomic baseline; no race.

### `onPropertyApplied` throw policy (RR-UE3D)

Spec'd in this ticket: `SectionEditForm` wraps the host callback in try/catch. On throw, log + toast (via `onError`), but **don't** roll back the local `formData` write. The local section reflects the server-confirmed value (which is the truth); the host's bug to fix its reconciler. Reasoning: rolling back creates inconsistency between section and server; better to have the section right and the host wrong than both wrong.

### `fieldVerdicts` flip semantics (RR-UE3G)

Spec'd: in-flight PATCHes complete (server is final arbiter). On verdict flip mid-flight, the cell re-renders in display mode showing `useAutoSave`'s `lastSeenServer[prop]`. Pending un-fired debounced edits are discarded with a toast ("permission changed — your unsaved changes were discarded").

### One indicator per section (RR-UE3H)

Spec'd: one `AutoSaveIndicator` per `SectionEditForm`. Concurrent saves across multiple cells in the same section show as a single saving state. Per-cell affordance is intentional scope reduction; can be added in a follow-up if real users find it confusing. Documented in deltas table.

## Known behaviour deltas (sketch — refine in planning)

| # | Surface | Before | After | Justification |
|---|---|---|---|---|
| 1 | Properties section with writable fields | Display-only via PropertyDisplay | SectionEditForm replaces PropertyDisplay; writable cells become editable in place | Net-new capability. Gated by `_fields`. |
| 2 | `_fields[prop].writable === false` cells | Rendered same as writable | Render in display mode inside SectionEditForm | Net-new affordance read. |
| 3 | Status indicator on properties section | (didn't exist) | One AutoSaveIndicator per SectionEditForm | New visual; consistent with form-side UX. |
| 4 | Permission flip mid-session | Did not surface | Toast on next refresh ("permission changed"); pending edits discarded | New affordance. |
| 5 | Brief viewData.entry inconsistency window | (didn't exist) | Between commit and onPropertyApplied: section shows new value; other on-page renderers (header Badge, related sections) show old value until callback writes back | ~600ms; status indicator gives user visible feedback. |

## Verification gate (sketch)

1. Per-widget mode-extension test — value-type round-trip per widget.
2. `SectionEditForm` unit tests — mount, schedule, apply, two-sources-of-truth, unmount-commit, `onPropertyApplied` throw, `fieldVerdicts` flip.
3. Two-sources-of-truth test (RR-UE2B / RR-UE3D) — property edited in section updates Badge rendering same property elsewhere on page.
4. `_fields` gate test — verdict flip mid-session results in correct semantic per RR-UE3G spec.
5. Browser smoke — edit a property in the entity properties section; observe autosave indicator; observe the header Badge updates.

## Inherited findings

Resolves:
- **RR-UE3A** (provide/inject) — by having `SectionEditForm` own the iteration.
- **RR-UE3C** (PropertyDisplay integration) — option (c) picked.
- **RR-UE3D** (`onPropertyApplied` throw) — spec'd: try/catch, no rollback.
- **RR-UE3G** (verdict flip semantics) — spec'd: in-flight complete; pending discarded with toast.
- **RR-UE3H** (indicator placement) — spec'd: one per section.
- **RR-UE3I** (per-widget tautological test) — sharpened to value-type round-trip assertion.

Depends on:
- **TKT-IHC7A** (per-channel debounce, `initialServerSnapshot`, channel disable on `useAutoSave`)

## Out of scope (deferred to TKT-IHC7C or further)

- Cards/list inline edit
- Wire-shape change for typed `_props` per cards/list entity
- View-config `editable: true` overrides (TKT-HOIX1)
- TKT-GUPMK markdown content body inline-edit

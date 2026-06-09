---
id: TKT-IHC7A
type: ticket
title: Per-channel debounce + checkbox-toggle to useAutoSave
kind: enhancement
priority: high
effort: s
status: done
---

## Goal

Migrate `EntityDetail.handleCheckboxToggle` from its bespoke PATCH+splice flow (with a `togglingIndices` dedupe Set) to `useAutoSave.scheduleContentSave`. This is the high-confidence, low-risk piece of the split TKT-IHCY7 — no new components, no widget contract changes, no view-side wire-shape changes.

In support of the migration, `useAutoSave` gains a small API extension:

- **Per-channel debounce options** (`fieldDebounceMs`, `contentDebounceMs`) so the checkbox-toggle path can debounce at ~100ms while the form side keeps its 800ms typing default.
- **`initialServerSnapshot` constructor option** so callers can seed `lastSeenServer` atomically without a separate `recordServerSnapshot` call (RR-UE3F).
- **Channel-disable flags** (`disablePropertyChannel`, `disableContentChannel`, `disableRelationsChannel`) so callers that only use one channel don't pay the cost of unused channel iteration in `mergeServerResponse` (RR-UE3J).

## Scope

### `useAutoSave` API extension

1. **Per-channel debounce.** Add `fieldDebounceMs?: number` and `contentDebounceMs?: number` to `AutoSaveOptions`. Default behaviour preserves the form side: when the legacy `debounceMs` is set, it applies to both channels (back-compat alias). When `fieldDebounceMs` / `contentDebounceMs` are set explicitly, they override per-channel. Defaults if nothing is set: 800ms each.

2. **`initialServerSnapshot` option.** Add `initialServerSnapshot?: Record<string, unknown>` to `AutoSaveOptions`. When set, `lastSeenServer` is seeded atomically during construction. This eliminates the race where a widget that emits on mount fires `scheduleFieldSave` before a separate `recordServerSnapshot` call lands.

3. **Channel-disable flags.** Add `disablePropertyChannel?: boolean` and `disableContentChannel?: boolean` and `disableRelationsChannel?: boolean` to `AutoSaveOptions`. When set, the corresponding `schedule*` functions throw a developer error if called, `mergeServerResponse` skips the iteration for the disabled channel, and `commitImmediately` skips its fire-step. Defaults: all `false` (preserves form-side behaviour).

### `EntityDetail` migration

Replace the existing `handleCheckboxToggle` (`EntityDetail.vue` L184-240) with a thin handler that calls `useAutoSave.scheduleContentSave` on a content-only instance:

```ts
const contentAutoSave = useAutoSave({
  getEntityType: () => entry.value?.type ?? '',
  getEntityId: () => entry.value?.id ?? '',
  formData: ref({}),
  contentRef: computed(() => entry.value?.content ?? ''),
  inverseToCanonical: new Map(),
  buildRelationsBody: () => null,
  contentDebounceMs: 100,
  disablePropertyChannel: true,
  disableRelationsChannel: true,
  initialServerSnapshot: undefined, // content channel doesn't use lastSeenServer for properties
  applyServerProperty: () => {}, // disabled
  applyServerContent: (newContent) => {
    const view = viewData.value
    if (!view) return
    const updated = { ...(view.entry as Entity), content: newContent }
    const nextSections = view.sections.map((s) =>
      isEntryContentSection(s) ? { ...s, content: newContent } : s,
    )
    viewData.value = { ...view, entry: updated, sections: nextSections }
  },
  onError: (msg) => uiStore.error(msg),
})

async function handleCheckboxToggle(index: number) {
  const current = entry.value
  if (!current) return
  try {
    const newContent = toggleCheckboxInSource(current.content || '', index)
    contentAutoSave.scheduleContentSave(newContent)
  } catch (err) {
    uiStore.error(`Failed to toggle checkbox: ${err instanceof Error ? err.message : 'unknown error'}`)
    console.error(err)
  }
}
```

The `togglingIndices` Set is removed. The composable's FIFO chain serializes content writes strictly stronger than per-index dedupe.

### What stays out of scope

- **Widget contract changes.** No `WidgetMode` widening. No `'inline-edit'` mode. No new emits.
- **New components.** No `SectionEditForm`. No `InlineEditField`.
- **`_fields` writability gating on the view side.** Not yet — IHCY7b's surface.
- **Cards/list inline edit.** TKT-IHC7C.
- **Properties-section inline edit.** TKT-IHC7B.

## Non-goals

- No view-config surface.
- No optimistic UI.
- No SSE-driven reconciliation.
- No ETag/If-Match.
- No backend changes.

## Why this ticket exists

TKT-IHCY7 was originally `m`-effort for "generalize inline-edit plumbing." Three rounds of cranky design-review surfaced 35 findings, with the round-3 reviewer noting: *"Two prior rounds dissolved problems by absorbing scope; this round can't absorb the wire-shape gap."*

The honest split:

- **TKT-IHC7A (this ticket)** — the high-confidence piece: tighten `useAutoSave` (per-channel debounce, initial snapshot, channel disable) and migrate the checkbox-toggle. Ships first.
- **TKT-IHC7B** — properties-section inline edit via `SectionEditForm`. Builds on this ticket. Resolves RR-UE3A and RR-UE3C.
- **TKT-IHC7C** — cards/list inline edit. Requires a wire-shape change to include typed `_props` per entity (RR-UE3B). Builds on IHC7B. May split further.

## Known behaviour deltas

| # | Surface | Before | After | Justification |
|---|---|---|---|---|
| 1 | Checkbox toggle in markdown content | Bespoke `togglingIndices` Set + custom PATCH+splice in `handleCheckboxToggle` | `useAutoSave.scheduleContentSave` with `contentDebounceMs: 100` and FIFO chain serialization | Same observable behaviour; stronger serialization; brief `AutoSaveIndicator` on toggle (visible ~600ms minimum). The 100ms debounce is below the typical e2e tolerance and below human-perception latency for click-to-feedback. |
| 2 | Rapid checkbox toggles on different checkboxes | Two PATCHes in flight (one per checkbox index), dedupe per-index | One PATCH in flight at a time (FIFO chain on the content channel) | Stronger serialization at the cost of small queuing latency for rapid different-checkbox clicks. The server's write mutex was already serializing these; the wall-clock difference is small. Avoids two PATCHes computing `newContent` from the same baseline and clobbering each other (an existing latent bug, not just a refactor concern). |
| 3 | Brief autosave status indicator | (didn't exist) | `AutoSaveIndicator` shows for ~600ms during/after PATCH | Inherited from the form-side autosave UX. Consistent. |

## Verification gate

1. **`useAutoSave` per-channel debounce unit tests.** Existing tests still pass; new tests verify `fieldDebounceMs` / `contentDebounceMs` independently control their channels; back-compat alias `debounceMs` still sets both.
2. **`useAutoSave` `initialServerSnapshot` test.** Construct with `initialServerSnapshot: { foo: 'a' }`, immediately schedule `foo = 'a'`, assert no PATCH (no-op suppression baseline).
3. **`useAutoSave` channel-disable tests.** Each disable flag tested: PATCHes only the enabled channels; `mergeServerResponse` skips the disabled channel; `commitImmediately` skips the disabled channel; calling the disabled channel's `schedule*` function throws.
4. **`EntityDetail` content-channel test.** Click a checkbox; assert one PATCH fires within ~100-200ms with the toggled content; assert `viewData.entry.content` is updated from the response; assert no second PATCH on a rapid second click before the first returns.
5. **Existing e2e/tests/checkboxes.spec.ts passes unchanged.** Read the test first; the 100ms debounce should not break it. If the test asserts a tighter timing window (unlikely but possible), tune `contentDebounceMs` lower or update the test (in which case it's a documented behaviour delta).
6. **DynamicForm tests pass unchanged.** The legacy `debounceMs` API still works; no DynamicForm changes.
7. **Browser smoke.** Toggle a checkbox in a ticket's markdown content; observe the brief status indicator; observe no flicker on rapid clicks.

## Out of scope (deferred — see TKT-IHC7B and TKT-IHC7C)

- Widget contract `'inline-edit'` mode
- `SectionEditForm` component
- Properties-section inline edit
- Cards/list inline edit
- View-side `_fields` writability gating
- Wire-shape change for typed `_props` per cards/list entity
- `PropertyDisplay` overhaul

## Inherited findings from TKT-IHCY7 (round-3)

This ticket resolves:

- **RR-UE3E** (per-channel debounce as a back-compat-respecting API extension)
- **RR-UE3F** (`initialServerSnapshot` constructor option for atomic seed)
- **RR-UE3J** (channel-disable flags)

These RRs are `deferred` on the parent TKT-IHCY7 with notes pointing to this ticket as their resolution scope.

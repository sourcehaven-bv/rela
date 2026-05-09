---
id: PLAN-L9NL2
type: planning-checklist
title: 'Planning: Per-property + content auto-save in DynamicForm'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**
- Backend: extend `PATCH /api/v1/{plural}/{id}` request body with `properties_unset: []string`. After applying `properties` upsert, delete the named keys from the entity's properties. Per DEC-HWZHA, unknown keys produce a `unknown_property_unset_key` warning (200 + warnings), not 422.
- Frontend `entities.ts`: extend the existing `updateEntity` patch type to accept `properties_unset?: string[]`. No new `patchEntity` function — single update API for the form.
- Frontend `useAutoSave` composable: per-property + per-content save queue with debounce, FIFO serialization, optimistic UI status (`idle` / `saving` / `saved` / `error`), revert affordance on failure.
- Frontend `dirtyFormRegistry` cross-route registry of currently-dirty fields so an inbound SSE `entity:updated` event does not clobber the user's in-progress edit.
- Frontend `AutoSaveIndicator.vue` widget: status pill + last-error tooltip + per-field revert button.
- DynamicForm integration: replaces the explicit Save button for forms whose widgets are auto-save-compatible. **Forms with RelationCards keep the Save button** until TKT-B9SXH lands the relation half — gate per-form via a `formAllowsAutosave(formConfig)` predicate.
- FieldRenderer integration: per-field error display + revert button next to the field when a save fails for that property.
- MarkdownEditor integration: hooks into the dirty bit and content save queue.
- Warnings consumption: when PATCH returns `200 + warnings: [{code, path, detail}]`, attach the warning to the addressed field as an inline non-blocking hint (rendered in yellow/info color), distinct from the error UX (red + revert button) reserved for 400/422.
- Navigation guard: `commitImmediately()` flushes the pending queue before route changes; if there are in-flight saves, the navigation is blocked until they settle. Prompts the user only on hard error.

**Out of scope:**
- Relation auto-save (RelationCards / RelationPicker) — TKT-B9SXH. Forms with RelationCards keep the explicit Save button.
- Conflict UX when an external edit wins (separate ticket if it ever materializes).
- Offline draft storage (Local-first tooling philosophy says no, for now).
- Multi-tab live collaboration.
- Optimistic concurrency tokens beyond the existing ETag.
- ID generation / create flow — autosave is for **edit** only. Create still requires explicit submit.

**Acceptance Criteria:**

### Property edits

1. **AC1 — Set property**: type into a property field. After `debounceMs` (default 800), a `PATCH /api/v1/{plural}/{id}` fires with body `{properties: {<field>: <value>}}`. Indicator transitions `idle → saving → saved`. **Test**: vitest unit test against `useAutoSave` mocking the API.
2. **AC2 — Clear property**: clear a property field (empty string for string types, etc.). PATCH fires with `{properties_unset: [<field>]}`. Indicator → saved. **Test**: vitest.
3. **AC3 — Two rapid edits to same field**: type, type again before debounce. Exactly ONE PATCH fires with the latest value. **Test**: vitest with fake timers.
4. **AC4 — Two edits to different fields**: type into A, then B before A's debounce. Two PATCHes serialize through the FIFO queue but the indicator UI feedback is per-field (A "saving" doesn't block B from showing "saving" too). **Test**: vitest.

### Content edits

5. **AC5 — Type in markdown body**: debounced PATCH with `{content: <body>}`. Indicator → saved. **Test**: vitest.
6. **AC6 — Clear body**: empty out the body. PATCH fires with `{content: ""}` (TKT-6WLSW pointer-vs-string semantics). **Test**: vitest.

### Failure modes

7. **AC7 — 422 on a property**: server returns 422. The field shows its error inline; a revert button restores the last server-known value; other fields keep working. **Test**: vitest with mocked 422 response.
8. **AC8 — Warnings on success**: server returns `200 + warnings: [{code, path, detail}]`. The warning attaches to its `path`-addressed field as an inline yellow hint, NOT a red error. Save indicator remains "saved". **Test**: vitest.
9. **AC9 — Network error**: offline / 500. Indicator shows error; revert button restores last server state. Periodic retry NOT in scope for v1; user must navigate away or revert. **Test**: vitest.

### SSE / dirty protection

10. **AC10 — SSE while typing**: `entity:updated` event arrives while user is mid-typing field A. Field A's dirty value is preserved; non-dirty fields B, C update from the event. **Test**: vitest with mocked SSE.
11. **AC11 — Dirty registry cross-route**: navigate from Form/A to Form/B; A's autosave instance unmounts but the dirty registry tracks no-longer-mounted fields long enough to commit them. (Bounded by the dirty window, default 1500ms.) **Test**: vitest unit on registry.

### Navigation guard

12. **AC12 — Navigate away with pending**: pending saves complete before route changes via `commitImmediately()`. If a hard error occurs during commit, user sees a confirm-or-cancel prompt (browser-native). **Test**: e2e (Playwright) — covered by an integration test rather than vitest.

### Form gating

13. **AC13 — RelationCards forms keep Save**: when a form's config includes a `cards` relation widget, the Save button is preserved and `useAutoSave` is not mounted. **Test**: vitest on DynamicForm with mocked formConfig.
14. **AC14 — Forms without RelationCards autosave**: a form with only field widgets uses autosave; no Save button visible. **Test**: vitest.

### Backend

15. **AC15 — Backend `properties_unset`**: PATCH with `{properties_unset: ["title"]}` removes `title` from the entity's properties on disk. **Test**: Go integration test against the existing in-memory store harness.
16. **AC16 — Unknown unset key warns**: PATCH with `{properties_unset: ["nonexistent"]}` returns 200 with a `warnings: [{code: "unknown_property_unset_key", path: "/properties_unset/0", detail: "..."}]`. The entity is unchanged for that key (silent no-op since it was already absent). **Test**: Go integration test.
17. **AC17 — Properties + properties_unset together**: PATCH with both `{properties: {title: "new"}, properties_unset: ["status"]}` upserts title AND clears status in one round-trip. **Test**: Go integration test.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Reference WIP**: local branch `wip/autosave-TKT-18JS6`, commit `097f64c` (2026-05-04). ~1300 lines of working frontend (`useAutoSave.ts` 458 lines, `useAutoSave.test.ts` 334 lines, `dirtyFormRegistry` + tests, `AutoSaveIndicator.vue`) + ~150 lines of backend (`properties_unset` + tests). Predates TKT-6WLSW so the WIP frontend uses an obsolete `patchEntity` function and the WIP backend has no warnings surface.
- **Ports verbatim from WIP** (logic is wire-format-independent):
  - `useAutoSave.ts` core composable — debounce, FIFO queue, optimistic UI, dirty interaction, revert. Replace internal `patchEntity` calls with `entitiesStore.update`.
  - `useAutoSave.test.ts` — tests the composable behavior, agnostic to the wire format.
  - `dirtyFormRegistry.ts` and `.test.ts` — self-contained registry.
  - `AutoSaveIndicator.vue` — UI widget.
  - `MarkdownEditor.vue` integration delta (~12 lines).
- **Ports with rework** (changed on develop since WIP):
  - `DynamicForm.vue` — current state has changed (993 lines vs WIP's smaller version). Re-walk integration: replace `handleSubmit` for autosave-eligible forms; keep it for RelationCards forms; add `formAllowsAutosave` predicate; mount `AutoSaveIndicator`.
  - `FieldRenderer.vue` — current state is 339 lines. Add per-field error / warning display and revert button.
  - `internal/dataentry/api_v1.go` — current handler has changed since WIP (8-line diff is no longer a clean apply). Re-write the small backend addition.
  - `internal/dataentry/api_v1_test.go` — port the 141 lines of tests, updating any test-harness changes.
- **Ports as net new under TKT-6WLSW's policy**:
  - `unknown_property_unset_key` warning — DEC-HWZHA-aligned soft-condition surface, didn't exist in WIP.
  - Frontend warning consumption code paths — WIP didn't have them.
- **Reference vs cherry-pick**: do NOT git-cherry-pick `097f64c`. The WIP branch is 4-stale-commits-old beneath that commit (the mobile-responsive drafts that have since shipped under different SHAs). Cherry-pick conflicts on 4 files. Instead read the WIP files via `git show 097f64c:<path>` and adapt.
- **rela concepts**: `FEAT-XN6JX` "Form auto-save with optimistic UI" is the parent feature. `TKT-6WLSW` shipped the wire format this consumes. `DEC-HWZHA` governs the warnings vs errors split.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Layer 0 — Backend `properties_unset`** (`internal/dataentry/api_v1.go`):

Extend the request struct in `handleV1UpdateEntity`:

```go
var req struct {
    Properties      map[string]interface{} `json:"properties,omitempty"`
    PropertiesUnset []string               `json:"properties_unset,omitempty"`
    Content         *string                `json:"content,omitempty"`
    Relations       V1RelationsField       `json:"relations,omitempty"`
}
```

Apply order: properties merge → properties_unset delete → content replace. Per
DEC-HWZHA, surface a `unknown_property_unset_key` warning (200 + warning) when a
key in `properties_unset` is not declared on the entity type's metamodel. The
warning's `path` is `/properties_unset/<index>`. The actual delete is silently
no-op for unknown keys — same as the legacy reconciler today.

Tests: AC15, AC16, AC17 in `internal/dataentry/api_v1_test.go`.

**Layer 1 — Frontend `entities.ts`** (`frontend/src/api/entities.ts`):

Extend the patch type used by `updateEntity` (no new function):

```typescript
export interface UpdateEntityPatch {
  properties?: Record<string, unknown>
  properties_unset?: string[]
  content?: string
  relations?: Record<string, RelationsUpdate | string[]>
}

export async function updateEntity(
  type: string, id: string, patch: UpdateEntityPatch, etag?: string,
): Promise<Entity> {
  return api.patch<Entity>(`/${getPlural(type)}/${id}`, patch, etag)
}
```

Add a `Warning` type matching the backend's response shape:

```typescript
export interface Warning { code: string; path?: string; detail?: string }
```

Extend `Entity` type to include `warnings?: Warning[]`.

**Layer 2 — `dirtyFormRegistry`**
(`frontend/src/components/forms/dirtyFormRegistry.ts`):

Port verbatim from WIP. A module-scoped `Map<entityID, Set<fieldKey>>` plus
`subscribe(entityID, listener)` callback set. Stays alive across route changes
(module scope, not Vue instance scope) so a navigation away from Form/A doesn't
lose A's dirty marker before the queued PATCH fires.

**Layer 3 — `useAutoSave` composable**
(`frontend/src/composables/useAutoSave.ts`):

Port from WIP with the following deltas:

1. **Wire-format substitution**: replace WIP's `patchEntity(type, id, patch)` with `entitiesStore.update(type, id, patch)`. Behavioral equivalence — both call the same backend.
2. **Warnings consumption**: when the response includes `warnings: [{code, path, detail}]`, the composable categorizes each by its `path`:
   - Path matches `/properties/<field>` or `/properties_unset/<index>` → attach to that field as a yellow hint via `fieldWarnings.value[field] = { code, detail }`. Indicator stays "saved".
   - Path matches `/content` → attach to content as a yellow hint via `contentWarning.value`.
   - Other paths → log to console; no UI surface (relations warnings are handled by TKT-B9SXH).
3. **Symbol sentinel for unset**: the WIP uses an internal `Symbol("unset")` sentinel to mark a field as "queued for unset" in the per-field pending map; preserve.
4. **`commitImmediately()`**: returns a Promise that resolves when the pending queue drains. Used by the navigation guard.

Public API the composable exposes:

```typescript
{
  status: ComputedRef<'idle' | 'saving' | 'saved' | 'error'>,
  lastError: ComputedRef<string | null>,
  inFlightCount: ComputedRef<number>,
  pendingCount: ComputedRef<number>,
  fieldErrors: ComputedRef<Record<string, string>>,
  fieldWarnings: ComputedRef<Record<string, Warning>>,  // new vs WIP
  contentError: ComputedRef<string | null>,
  contentWarning: ComputedRef<Warning | null>,  // new vs WIP
  isDirty: (field: string) => boolean,
  isContentDirty: () => boolean,
  scheduleFieldSave: (field: string, value: unknown) => void,
  scheduleUnset: (field: string) => void,
  scheduleContentSave: (content: string) => void,
  commitImmediately: () => Promise<void>,
  revertField: (field: string) => void,
  revertContent: () => void,
  recordServerSnapshot: (entity: Entity) => void,
  mergeServerResponse: (entity: Entity) => void,
}
```

**Layer 4 — `AutoSaveIndicator.vue`**
(`frontend/src/components/forms/AutoSaveIndicator.vue`):

Port verbatim. Renders a small status pill (idle / saving / saved / error) plus
a tooltip on the last error. No revert affordance here — that lives next to the
offending field via FieldRenderer.

**Layer 5 — `FieldRenderer.vue` integration**:

Inject `fieldErrors` and `fieldWarnings` from the composable (via prop or
provide/inject). For each field:
- If `fieldErrors[field]` is set → render a red error message + revert button.
- Else if `fieldWarnings[field]` is set → render a yellow hint.
- Else → no inline state.

**Layer 6 — `DynamicForm.vue` integration**:

Add a `formAllowsAutosave(formConfig)` predicate: returns false if any widget is
`cards` or `relations` (RelationCards / RelationPicker — TKT-B9SXH territory).
Returns false in create mode.

When predicate is true:
- Mount `useAutoSave` with the entity's type/id.
- On every property change in formData, call `scheduleFieldSave` (or `scheduleUnset` for empty values).
- On every content change, call `scheduleContentSave`.
- Render `AutoSaveIndicator` instead of the Save button.
- Wire the Vue Router beforeEach guard to call `commitImmediately()` then await its promise before allowing the route change.

When predicate is false:
- Keep current explicit-Save behavior unchanged.

**Layer 7 — `MarkdownEditor.vue` integration**:

Wire `scheduleContentSave` on debounced content changes. Honor `contentError` /
`contentWarning` for inline display.

**Files to modify:**

- `internal/dataentry/api_v1.go` — add `PropertiesUnset` field; apply delete loop; emit warning for unknown keys
- `internal/dataentry/api_v1_test.go` — AC15, AC16, AC17
- `frontend/src/api/entities.ts` — extend `updateEntity` patch type with `properties_unset`; add `Warning` type
- `frontend/src/types/entity.ts` (or wherever `Entity` lives) — add optional `warnings?: Warning[]`
- `frontend/src/composables/useAutoSave.ts` (new, ~470 lines after warnings additions)
- `frontend/src/composables/useAutoSave.test.ts` (new, ~340 lines)
- `frontend/src/components/forms/dirtyFormRegistry.ts` (new, ~46 lines)
- `frontend/src/components/forms/dirtyFormRegistry.test.ts` (new, ~62 lines)
- `frontend/src/components/forms/AutoSaveIndicator.vue` (new, ~146 lines)
- `frontend/src/components/forms/DynamicForm.vue` — autosave integration, gating by formAllowsAutosave predicate, navigation guard
- `frontend/src/components/forms/FieldRenderer.vue` — per-field error/warning + revert
- `frontend/src/components/forms/MarkdownEditor.vue` — content save hookup
- `CLAUDE.md` — autosave note in the data-entry section
- `frontend/CLAUDE.md` — composable + dirty registry + indicator note

**Alternatives considered:**

- **Add a parallel `patchEntity()` function in entities.ts** (as the WIP did): rejected. One save function for the form is cleaner; the WIP's parallel function predates TKT-6WLSW and was a workaround.
- **Cherry-pick `097f64c` and resolve**: rejected. 4-file conflict against current develop and the WIP branch carries 4 stale mobile-responsive commits beneath it. Cleaner to read the WIP and re-author.
- **Treat all PATCH responses uniformly (no warning vs error split)**: rejected. The whole point of TKT-6WLSW's warnings surface is to give UIs non-blocking feedback. Mapping warnings to red error UX would defeat that.
- **Ship without the navigation guard**: rejected. Without the guard, a user clicking away just before debounce fires loses the keystroke. That's the *exact* bug autosave is supposed to fix.
- **Per-field independent in-flight saves (no FIFO)**: rejected. Two PATCHes for two fields could land on the server out-of-order; if one is "set X then unset X" the wire-arrival order matters. FIFO serialization is correct.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`properties_unset` array values** (HTTP request): each entry is a property name. The backend deletes from a Go `map[string]interface{}` — no path traversal surface. Unknown keys produce a warning per DEC-HWZHA, not 422. RFC 6901 escaping applies to the warning's `path` for keys with special characters.
- **Frontend payload construction**: the composable builds `{properties_unset: [<field>]}` from `Object.keys(formData)` filtered to the current field. No user-controlled field names from outside the form's own metamodel-driven schema.
- **No new auth surface, no new file-system surface, no crypto.**

**Error handling:**

- 4xx errors render the server's `detail` string inline next to the field. The server already sanitizes `detail` (no request body bytes echoed); this PR doesn't change that.
- Network errors render a generic "Save failed; try again or revert" message. Stack traces never reach the UI.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see AC1–AC17 above. Backend ACs (15–17) are Go integration
tests in `internal/dataentry/api_v1_test.go`. Frontend ACs (1–14) are vitest
unit tests against the composable plus a small set of DynamicForm integration
tests; AC12 (navigation guard) is an e2e in Playwright.

**Edge Cases:**

- **Type into field, immediately revert**: composable's pending entry is cleared; no PATCH fires.
- **PATCH succeeds while user is typing again**: server response merges into form state ONLY for fields that aren't currently dirty; dirty fields are preserved (the dirty registry).
- **Multi-line markdown body**: debounce fires on the content channel separately from property fields. Both can be in-flight simultaneously through the FIFO queue.
- **Field set to a falsy value (false / 0 / "")**: distinguish from "cleared". A boolean flipping to false sends `{properties: {flag: false}}`, NOT `{properties_unset: ["flag"]}`. The unset path is reserved for the user explicitly emptying a field where empty means "remove".
- **Schema-required field cleared**: the backend's `validateEntity` reports a validation error (this is a hard-422 today on `entityManager.UpdateEntity`). The composable surfaces it as a field error with revert button. Auto-save WORKS — the save fails, user sees the error, can revert.
- **`isCreate` mode**: composable is not mounted; explicit Save remains.
- **ETag race**: existing 412 path still applies. Composable surfaces it as an error.
- **Concurrent two browser windows**: SSE event from window 2 reaches window 1's dirtyFormRegistry; if the field is dirty, the change is held; if not, it merges in.

**Negative Tests:**

- AC7 — 422 on a property: assert error rendered, revert restores last server value.
- AC9 — Network error: assert error rendered, no infinite retry.
- AC16 — Unknown `properties_unset` key: assert 200 + warning, entity unchanged.
- Field with malicious value (script tag in a string): backend stores as-is; frontend renders via Vue's text interpolation (XSS-safe). Existing behavior; not changed.

**Integration test approach:**

- Backend: existing `internal/dataentry/api_v1_test.go` harness (`newTestAppV1`).
- Frontend unit: vitest with `vi.useFakeTimers()` for debounce control, Mock Service Worker for the API responses.
- Frontend e2e (AC12 only): Playwright, real browser, real backend. This is the only AC where the navigation-guard timing matters.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Risk: DynamicForm regression — autosave-eligible vs not detection misclassifies a form, breaking either save flow.**
   - **Mitigation**: explicit `formAllowsAutosave` predicate with a unit test enumerating widget types. Forms with `cards` widget keep current Save behavior; forms without it autosave. AC13 and AC14 lock this in.
2. **Risk: dirty registry leaks fields across routes when the user navigates without committing.**
   - **Mitigation**: `commitImmediately()` runs before route change; bounded `dirtyWindowMs` (1500ms) on the registry expires stale entries. AC11 + AC12.
3. **Risk: SSE event arrives mid-PATCH, frontend merges stale data over the just-saved value.**
   - **Mitigation**: `mergeServerResponse` skips fields that are dirty OR have a pending entry in the FIFO queue. The dirty window covers the gap between "save fired" and "next SSE event arrives reflecting the save". AC10.
4. **Risk: `properties_unset` of a required field on an entity puts it in a permanent validation-error state.**
   - **Mitigation**: this is by design — the entity is invalid until the user re-fills the field, and `analyze_validations` flags it. AC7's revert button lets the user back out. The backend write path doesn't reject (DEC-HWZHA: tolerate temporarily invalid data).
5. **Risk: navigation guard blocks the user indefinitely on a stuck save.**
   - **Mitigation**: `commitImmediately` has a timeout (default 10s); on timeout, navigation proceeds with a confirm-or-cancel prompt.
6. **Risk: backwards-compat — adding `properties_unset` to the wire format silently changes legacy callers' behavior.**
   - **Mitigation**: purely additive — absent field = no-op delete loop. No existing caller sends `properties_unset` today.
7. **Risk: WIP code is 6 months old; copy-paste introduces subtle bugs against current develop.**
   - **Mitigation**: re-author rather than cherry-pick. Each ported file gets a fresh test pass. Tests are part of AC15-17 (backend) and embedded in vitest specs (frontend).
8. **Risk: per-field debounce + FIFO queue introduces race where field A's older value lands AFTER field B's newer value.**
   - **Mitigation**: the queue is global per entity; fires are serialized; the WIP composable already handles this. AC4 + AC5 prove it.

**Effort: m** — backend is xs (~10 lines + tests), frontend is m (port 1300
lines, rework integration in 3 vue files, add warnings consumption). Net 1.5–2
days of coding, plus design-review iteration.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/data-entry/api-reference.md` adds the `properties_unset` field section + the `unknown_property_unset_key` warning code
- [x] CLI help text — N/A
- [x] CLAUDE.md — add an "Auto-save in DynamicForm" subsection in data-entry
- [x] `frontend/CLAUDE.md` — composables + components index update for `useAutoSave`, `dirtyFormRegistry`, `AutoSaveIndicator`
- [x] README.md — N/A
- [x] API docs — `internal/openapi/openapi.yaml` regenerated for `properties_unset`
- [ ] N/A — Internal change, no user-facing docs needed

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** *<!-- to be filled after /design-review -->*

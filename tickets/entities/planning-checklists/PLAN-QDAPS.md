---
id: PLAN-QDAPS
type: planning-checklist
title: 'Planning: Auto-save for data-entry forms (auto_save: true)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:**

Today every form change requires the user to click Save. For journal-style or
list-style screens (e.g. the upcoming daily-notes UX) this is intrusive: ticking
a task, reordering items, or jotting a note shouldn't require a save click.
Forms should be able to auto-persist their changes.

This ticket introduces an opt-in auto-save mode that any form can enable. It
does *not* change behavior for forms that don't opt in.

**Scope (IN):**

- New `auto_save: true` flag on the `Form` config (default false).
- When enabled on an *edit* form: per-property debounced PATCH to the existing
entity-update endpoint (`/api/v1/{plural}/{id}`), serialized via a single
per-entity FIFO promise chain (NOT per-field — see RR-WU6NT).
- "Saving" / "Saved" / "Save failed" status indicator replaces the Save button.
One global indicator, not per-field.
- Per-property optimistic UI: change shows immediately. On 422, revert iff the
failed PATCH represents the latest user intent (see RR-UIL8J).
- **PATCH response merged back into formData** for non-dirty properties so
automation-derived values become visible without a page reload (RR-J7EZX).
- **New backend `properties_unset: ["x"]` field on the PATCH request** so the
client can express "delete this property" cleanly. The current pure-merge
semantics will still be used for set operations (RR-RDIQL).
- **SSE-driven refresh of formData** on `entity:updated` for the form's entity:
refetch via `entitiesStore.fetchEntity(type, id, force=true)` and merge into
`formData` / `relations` / `content` with rule "server wins for non-dirty
properties, local wins for dirty" (RR-P7E24).
- Dirty-property registry (`Map<entityId, Set<{ isDirty: (prop) => boolean }>>`)
consulted by both the SSE refresh path and the response-merge path (RR-Z5PQ2).
- `Cmd+Enter` / Save button hidden when `auto_save: true`.
- `beforeunload` and `onBeforeRouteLeave`: synchronously call
`commitImmediately()` first, then warn iff there is at least one in-flight PATCH
(RR-JPGU6).
- No-op suppression in `useAutoSave`: if the new value equals the last-seen
server value, skip the PATCH entirely. Bounds automation re-runs (RR-VKS1F).
- Defined error-class behavior:
  - 422 → revert (if latest intent) + sticky toast.
  - All other 4xx (404, 412, 400, 401, 403) → keep value, sticky toast,
`status='error'` until next successful save.
  - 5xx + network failures → keep value, sticky toast, `status='error'`.
  - No auto-retry for any class (RR-30SPD).
- Backend Form struct: `AutoSave bool \`yaml:"auto_save" json:"auto_save,omitempty"\``;
TS type uses `auto_save?: boolean` (snake_case, matching existing convention)
(RR-ZSJ99).
- Form-root `focusout` (with microtask delay so focus moving between widgets
doesn't fire) bound to `commitImmediately()` (RR-63I8M).
- MarkdownEditor: skip the `modelValue` watcher's external sync while the
editor is focused, OR accept a `skipExternalSync` prop the form sets while the
content field is in its dirty window (RR-USD3C).

**Scope (OUT):**

- **RelationCards autosave** — split out to TKT-B9SXH. The current
`cards-changed` flow remains unchanged in this ticket; auto-save mode forms with
relation-cards fields will fall back to the deferred-save behavior until
TKT-B9SXH lands. Documented as a known limitation.
- Create flow: auto-save kicks in only after the entity exists. Create still
uses an explicit submit.
- `relation-list` widget, `order_by`, `type: order` — separate tickets.
- Conflict detection / ETag enforcement — pre-existing gap; auto-save makes
it more visible but doesn't widen it. Tracked as follow-up risk.

**Acceptance Criteria:**

1. **Opt-in only**: a form *without* `auto_save: true` saves exactly as today
(Save button, dirty checks, route guards). Test: existing `forms.spec.ts` passes
unchanged.

2. **Per-property PATCH on edit**: with `auto_save: true`, editing one property
fires one PATCH containing only that property after debounce. Test: Vitest spy
on the API client, assert payload shape `{properties: {only_one: ...}}`.

3. **Debounce coalesces rapid edits**: three keystrokes within the debounce
window (default 800ms) fire one PATCH, not three. Test: Vitest fake-timers.

4. **Cross-field FIFO ordering**: PATCH for property X must complete before
PATCH for property Y begins, regardless of which was queued first. Test: Vitest
with API mock that delays PATCH A by 1000ms; queue PATCH B at 200ms; assert B
does not fire until A's promise resolves (RR-WU6NT).

5. **Optimistic UI + revert-on-422**: after a change, the field updates
immediately. On 422 for the *latest* PATCH, the value reverts and an error toast
is shown. If a newer keystroke has been queued or sent before the 422 arrives,
do NOT revert (newer intent supersedes). Test: Vitest with mocked API returning
422; both the "no superseding edit" case (revert) and the "superseding edit"
case (no revert) (RR-UIL8J).

6. **Non-422 4xx and 5xx keep value**: 404, 412, 400, 401, 403, 5xx, network
failure → user value preserved, sticky toast, status='error'. Test: Vitest
parameterized over each status code (RR-30SPD).

7. **Saving indicator**: while a PATCH is in-flight, indicator shows "Saving…";
on success "Saved" briefly; on failure "Save failed" (sticky until next
success). Test: Playwright asserts indicator transitions.

8. **PATCH response merged back into formData**: when an automation sets a
property as a side effect (e.g. `completed_at` when `status='done'`), the form's
display refreshes that property. The merge respects the dirty registry —
properties currently being edited locally are not overwritten. Test: Vitest with
a mocked PATCH response that includes a property the user didn't set; assert
that property shows in `formData` after save (RR-J7EZX).

9. **SSE-during-edit preserves dirty fields**: mount form on E, type 'abc'
into field X (no debounce advance), trigger SSE `entity:updated` for E, mock API
to return E with X='serverValue'. Assert `formData.X === 'abc'`. Then advance
debounce; PATCH fires; assert formData.X reflects user value. Converse: SSE for
E with field Y not dirty and server value 'newY'; assert `formData.Y === 'newY'`
after refresh (RR-7L0SW).

10. **`properties_unset` clears properties cleanly**: clearing a non-required
optional property sends `{properties_unset: ["x"]}` (not `{x: null}` or `{x:
""}`). After PATCH, the entity's YAML front matter no longer contains that key.
Test: Go test on the new endpoint code path; Vitest test that clearing the field
produces the right payload (RR-RDIQL).

11. **No-op suppression**: setting a property to the same value it already
has on the server does not fire a PATCH. Test: Vitest setting `formData.x =
serverValue.x` and asserting no API call (RR-VKS1F).

12. **`beforeunload` flushes pending changes**: type into a field, trigger
`beforeunload` before debounce fires. Assert the queued PATCH is sent
synchronously via `commitImmediately()`. Test: Vitest with `dispatchEvent` on
beforeunload; Playwright e2e on a real page reload (RR-JPGU6).

13. **Two DynamicForm instances on same entity**: side panel + main page on
the same entity, both in auto-save. Both forms register independently in the
dirty registry; SSE refresh skips properties dirty in *either*. Test: Vitest
mounting two forms on the same entityId, asserting both callbacks invoked
(RR-Z5PQ2).

14. **MarkdownEditor caret preserved across response merge**: type a paragraph,
response merge fires (or simulated SSE refresh), assert caret position and
content unchanged while editor is focused. Test: Playwright on a form with
content body (RR-USD3C).

15. **No regression on create**: creating a new entity uses explicit submit
even with `auto_save: true` in the form config. Test: Playwright creating a new
entity.

16. **Save button absent in auto-save edit mode**: Playwright assertion on
absence of `[data-testid="save-button"]`.

17. **JSON wire format snake_case**: GET `/api/v1/_config` response includes
`auto_save: true` (not `AutoSave`); TS type matches. Test: Go test on the
V1Config response; TS type-check (RR-ZSJ99).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

*Libraries considered:*

- **VueUse `useDebounceFn`** — repo doesn't depend on `@vueuse/core`. Inline
pattern at `FilterBar.vue:135-142` is the local convention; mirror it.
- **FormKit / vee-validate auto-save plugins** — full form-validation libs
that would reframe how all forms work. Rejected.
- **Yjs / Automerge** — multi-user CRDT libs. Out of scope; last-write-wins.

*Codebase patterns to reuse:*

- `frontend/src/components/lists/FilterBar.vue:135-142` — debounce pattern.
- `frontend/src/api/entities.ts:30-37` — `entitiesStore.update(type, id, payload)`,
partial PATCH already supported.
- `frontend/src/composables/useEvents.ts:130-140` — SSE handler. Currently
`invalidateAll`. We add a per-entity refetch hook for forms.
- `frontend/src/stores/ui.ts:74` — `uiStore.error` for sticky toasts.

*Backend confirmed reusable (mostly):*

- `internal/dataentry/api_v1.go:453` `handleV1UpdateEntity` accepts partial
PATCH. **Needs extension**: add `properties_unset []string` to the request to
support the "delete property" semantic (RR-RDIQL). Implementation: after the
merge loop, iterate `properties_unset` and `delete(entity.Properties, k)`.
- `internal/dataentry/watcher.go:241` SSE broker emits `entity:updated`
to all clients. No echo suppression — handled client-side.

*Prior art in rela:*

- BUG-005, FEAT-014 — markdown editor input flow.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach:**

### Backend (small, additive)

1. Extend `V1UpdateEntityRequest` (api_v1.go:475) with
`PropertiesUnset []string \`json:"properties_unset,omitempty"\``.
2. In `handleV1UpdateEntity` after the merge loop, for each key in
`PropertiesUnset`, `delete(entity.Properties, k)`. Validation in
`meta.ValidateEntity` then runs as today (so unsetting a required property still
422s). Persists empty front-matter key (key removed, not blanked).
3. Add `AutoSave bool \`yaml:"auto_save" json:"auto_save,omitempty"\``to`dataentryconfig.Form` (config.go). Pass through in V1Config response
(api_v1.go:923-944).
4. Tests: Go test on `handleV1UpdateEntity` for properties_unset (success
case + required-property 422 case + unknown-property tolerated case).
5. **Idempotence audit (per RR-VKS1F)**: walk current metamodel automations
in `prototypes/data-entry/*/metamodel.yaml`. Any non-idempotent ones gain a
brief note in the docs. No code changes here unless we find a real bug.

### Frontend — composables and registry

6. **`frontend/src/composables/useAutoSave.ts`** — single composable owning:
   - Per-entity save queue (one promise chain, shared across all properties
and content). FIFO ordering guaranteed.
   - Per-property debounce timers.
   - `lastSeenServer: Record<string, unknown>` snapshot for no-op suppression
and revert ground truth.
   - `pendingValue: Record<string, { value, enqueuedAt }>` so 422 reverts
resolve to the right snapshot per RR-UIL8J.
   - Methods:
     - `scheduleFieldSave(prop, value)` — debounced.
     - `scheduleContentSave(content)` — debounced.
     - `scheduleUnset(prop)` — debounced; uses `properties_unset`.
     - `commitImmediately()` — flushes all queued debounces synchronously.
     - `mergeServerResponse(props, content)` — merges PATCH/refresh response
into formData, skipping any property the dirty registry reports dirty.
     - `isDirty(prop)` — true if pending, in-flight, or within
`dirtyWindowMs` of last commit.
   - Reactive: `status: 'idle' | 'saving' | 'saved' | 'error'`,
`inFlightCount`, `pendingCount`, `lastError`.

7. **`frontend/src/components/forms/dirtyFormRegistry.ts`** — module-level
`Map<entityId, Set<DirtyCheck>>` where `DirtyCheck = (prop: string) => boolean`.
   - `registerForm(entityId, check)` returns `unregister()` for cleanup.
   - `anyFormDirty(entityId, prop)` — returns true if *any* registered
callback says dirty.
   - Set semantics handle two-form-on-same-entity case (RR-Z5PQ2).
   - Unit-tested for repeated mount/unmount cycles (HMR concern).

8. **SSE refresh hook**. In `DynamicForm`, subscribe to the SSE event stream
directly (don't bolt onto `useEvents.ts` which is a global cache layer). On
`entity:updated` matching `props.entityId`, call
`entitiesStore.fetchEntity(type, id, force=true)` and pass response to
`useAutoSave.mergeServerResponse(...)`.
   - This *adds* SSE-driven refresh that doesn't currently exist (per
RR-P7E24); the dirty-skip merge is the safety mechanism.

9. **`frontend/src/components/forms/AutoSaveIndicator.vue`** — small, three
visual states keyed on `useAutoSave.status` and `lastError`.

### Frontend — DynamicForm integration

10. **`DynamicForm.vue` changes** when `formConfig.auto_save && entityId`:
    - Replace Save button slot with `<AutoSaveIndicator>`.
    - In `updateField(prop, value)`:
      - If `value === undefined || value === ""` and the property is optional,
call `useAutoSave.scheduleUnset(prop)`. Else `scheduleFieldSave`. (Decide
per-property whether `""` means unset based on `propertyDef.type`.)
      - Mutate `formData[prop]` optimistically.
    - In `updateContent`, same with `scheduleContentSave`.
    - **Don't change `updateRelationCards` in this ticket** — it stays as
today's deferred-save behavior. Document the limitation.
    - Bind `commitImmediately()` to form-root `focusout` with microtask delay
(RR-63I8M).
    - Replace `beforeunload`/`onBeforeRouteLeave` handlers: synchronously call
`commitImmediately()`; warn iff `useAutoSave.inFlightCount > 0` after flush
(RR-JPGU6).

11. **`MarkdownEditor.vue`** — extend to skip the `modelValue` watcher's
external sync iff the editor is focused. Add a quick test that fires a prop
change while focused and asserts caret/content unchanged (RR-USD3C).

12. **`frontend/src/types/config.ts`** — add `auto_save?: boolean` to `Form`
(snake_case to match existing convention) (RR-ZSJ99).

13. **`frontend/src/api/entities.ts`** — add `unsetEntityProperties` helper
or extend `updateEntity` signature to accept `properties_unset`. Keep the wire
shape clean: `{properties?, properties_unset?, content?}`.

### Race / state diagrams (worth getting right before coding)

- **Per-entity FIFO queue**: every save (property X PATCH, property Y PATCH,
content PATCH, unset Z PATCH) appends to the same promise chain. A new call
awaits the chain's tail, then sends, then resolves the chain.
- **422 revert decision tree**:
  - Failed PATCH had property X = "abc", enqueuedAt = T1.
  - Current `pendingValue.X` is either: (a) absent (no newer edit) → revert
to `lastSeenServer.X`; (b) present with `enqueuedAt > T1` → don't revert.
  - Always show a sticky toast.
- **No-op suppression timing**: check happens at debounce-fire time, not at
schedule time, so rapid back-and-forth typing where the user lands back on the
server value coalesces to no PATCH.

**Files to modify:**

Backend:
- `internal/dataentry/api_v1.go` (~line 453, 475, 923) — PATCH semantics +
V1Config.
- `internal/dataentry/api_v1_test.go` — properties_unset tests.
- `internal/dataentryconfig/config.go` — `AutoSave` field.
- `internal/dataentryconfig/validate.go` (+ test) — optional warning.

Frontend:
- `frontend/src/composables/useAutoSave.ts` — new (+ test).
- `frontend/src/components/forms/dirtyFormRegistry.ts` — new (+ test).
- `frontend/src/components/forms/AutoSaveIndicator.vue` — new.
- `frontend/src/components/forms/DynamicForm.vue` — integrate.
- `frontend/src/components/forms/MarkdownEditor.vue` — focus-aware sync.
- `frontend/src/types/config.ts` — add `auto_save?`.
- `frontend/src/api/entities.ts` — `properties_unset` support.
- `frontend/e2e/forms-autosave.spec.ts` — new e2e suite.
- `prototypes/data-entry/*/data-entry.yaml` — add a form variant with
`auto_save: true` for e2e to exercise.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:**

- *PATCH bodies*: same as existing form-submit path. Server validation
unchanged.
- *`properties_unset` array*: server treats unknown keys as no-ops (delete on
a missing key is a no-op in Go). No injection vector — keys are not interpolated
into queries; they're map keys.
- *`auto_save` config flag*: parsed boolean from YAML.

**Security-sensitive operations:**

- The new `properties_unset` field could be abused to clear required
properties — but `meta.ValidateEntity` runs after the merge+unset and rejects
with 422. The user sees a revert. Same security posture as blanking a required
field via `properties: {x: ""}` today.

**Error-handling note:** sticky toasts surface server-supplied detail (already
truncated/styled by `uiStore.error`). No raw stack traces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

(See acceptance criteria 1–17 above for criterion-to-test mapping.)

**Edge cases (additional, beyond the AC list):**

- Repeated focus-blur cycles: blur should `commitImmediately()`.
- Markdown editor focused while SSE refresh fires: editor watcher must skip.
- User rapidly toggles a checkbox 5×: no-op suppression should fold to
zero or one PATCH.
- Two browser tabs editing different fields of the same entity: each tab's
SSE refresh updates non-dirty fields only.
- Backend 5xx mid-typing: indicator shows error, value preserved, next
successful save clears.
- `properties_unset` for an unknown key: 200, no-op (don't 4xx for forward-
compat with older clients).
- Optional property with `default: "foo"` cleared by user: should send
`properties_unset: [...]`, not `properties: {x: "foo"}`.

**Manual QA (Puppeteer-driven, on the actual dev server):**

A second-pass full QA after implementation. Documented as a separate phase of
the implementation checklist:

- All form widget types: text, select, multi-select, checkbox, textarea,
number, date, rrule, markdown content. For each: type, debounce, observe PATCH
in network tab, verify persistence after reload.
- Concurrent edits: open same entity in two tabs, edit different fields
simultaneously, verify both persist.
- Concurrent edits same field: edit field in two tabs, verify last-write-wins
with no JS errors.
- Rapid typing stress: hold key for 5s, count PATCHes, expect ≤ ~7 (5s/800ms).
- Network throttling: slow 3G profile, verify FIFO queue serializes correctly.
- Server crash mid-save: kill rela-server during a PATCH, verify error
surfaces, restart server, verify next save works.
- Validation rejection: edit a property to an invalid enum value, verify
revert + toast.
- Clear required property: try to clear, verify 422 revert.
- Clear optional property: verify front matter no longer contains the key.
- Tab switch / `commitImmediately`: type, switch tabs, verify save.
- Browser back/forward navigation: verify pending saves flush.
- Mobile / small screen: verify indicator visible, no layout regressions.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Per-entity FIFO queue stalls all saves on one slow request | Single global indicator clearly signals "saving"; document tradeoff. |
| SSE refresh races with optimistic merge | Dirty registry is the single source of truth — both paths consult it. Test races explicitly. |
| 422 revert clobbers newer keystrokes | `pendingValue` snapshots include `enqueuedAt`; revert decision tree only reverts when no superseding edit. |
| ETag pre-existing gap | Out of scope; auto-save doesn't widen it. Documented. |
| Automation fan-out | No-op suppression + audit. |
| MarkdownEditor caret loss | Focus-aware watcher in MarkdownEditor.vue + targeted test. |
| Existing tests break | Auto-save is opt-in. Existing forms unchanged. |
| RelationCards in auto-save mode batches to submit (no Save button → user can't trigger save) | Document as known limitation pending TKT-B9SXH. As short-term: when `auto_save: true` and the form has a relation-cards field, log a console warning during dev. |
| Two forms on same entity | `Map<entityId, Set<DirtyCheck>>`; tests cover the case. |

**Effort estimate:** **l** (large). Up from m after design review revealed real
coupling. Rough breakdown:

- 0.5d backend properties_unset + tests
- 0.5d backend AutoSave config plumbing
- 1.5d useAutoSave composable + per-entity FIFO + revert logic + tests
- 1d dirty registry + SSE refresh hook + tests
- 1d DynamicForm integration (button hide, focusout, beforeunload, response merge)
- 0.5d MarkdownEditor focus-aware watcher + test
- 0.5d AutoSaveIndicator component
- 1.5d Playwright e2e (`forms-autosave.spec.ts`) covering AC 1, 7, 9, 12, 14, 15, 16
- 1.5d Puppeteer-driven manual QA pass (the explicit "really try to break it" phase)
- 0.5d docs, code-review responses, polish

Total ~9d.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:**

- [x] User guide / reference docs — `auto_save` form config option;
`properties_unset` mention in API reference if present.
- [ ] CLI help text — N/A.
- [x] CLAUDE.md — data-entry section: document `auto_save`, the dirty
registry pattern, the FIFO queue.
- [ ] README.md — N/A.
- [x] API docs — document PATCH partial semantics + `properties_unset`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| RR | Severity | Status | How addressed in plan |
|----|----------|--------|-----------------------|
| RR-P7E24 | critical | addressed | Added explicit SSE refresh hook in DynamicForm + dirty-skip merge in `useAutoSave.mergeServerResponse`. Plan now states server-wins for non-dirty, local-wins for dirty. |
| RR-RDIQL | critical | addressed | Added `properties_unset: []string` to the PATCH wire format and backend handler. Frontend `scheduleUnset()` uses it. AC #10 validates. |
| RR-UPK8J | critical | addressed | Split RelationCards autosave to TKT-B9SXH. This ticket leaves `cards-changed` flow unchanged; documented limitation. |
| RR-30SPD | significant | addressed | AC #6 + scope spell out 4xx-other-than-422 → keep value, sticky toast. |
| RR-UIL8J | significant | addressed | `pendingValue` snapshots with `enqueuedAt`; revert only when no superseding edit. AC #5. |
| RR-JPGU6 | significant | addressed | `commitImmediately()` flushes synchronously before `beforeunload` decides; warn keys off `inFlightCount`, not status. AC #12. |
| RR-J7EZX | significant | addressed | `mergeServerResponse` runs after every PATCH response, dirty-skip enforced. AC #8. |
| RR-WU6NT | significant | addressed | Single per-entity FIFO queue (not per-property). One global indicator. AC #4. |
| RR-VKS1F | significant | addressed | No-op suppression in `useAutoSave` (skip PATCH if value matches lastSeenServer). AC #11. + automation audit task. |
| RR-USD3C | significant | addressed | MarkdownEditor's `modelValue` watcher made focus-aware. AC #14. |
| RR-ZSJ99 | significant | addressed | Explicit `json:"auto_save,omitempty"` on Go; TS uses snake_case. AC #17. |
| RR-7L0SW | minor | addressed | AC #9 strengthened with concrete assertions over `formData.X` and `formData.Y`. |
| RR-Z5PQ2 | minor | addressed | Registry shape is `Map<entityId, Set<DirtyCheck>>`. AC #13. |
| RR-63I8M | minor | addressed | `commitImmediately()` bound to form-root `focusout` with microtask delay. |
| RR-P7XKC | minor | addressed | Effort re-estimated; RelationCards split out (TKT-B9SXH). |

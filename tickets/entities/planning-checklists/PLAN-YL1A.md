---
id: PLAN-YL1A
type: planning-checklist
title: 'Planning: Unify delete confirmation on custom modal and wire Delete shortcut in list & detail views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

1. Extract the inline delete modal in `EntityView.vue` into a reusable
   `frontend/src/components/ui/ConfirmModal.vue` (generic confirm, not delete-specific).
2. Replace `window.confirm()` in `EntityList.vue` delete flow with `ConfirmModal`.
3. Wire `onDelete` in `EntityList.vue`'s existing `useListKeyboard` call so the
   `Delete` / `Backspace` keys open the modal for the selected row.
4. Extend `useListKeyboard.ts` to accept `Backspace` in addition to `Delete`
   (guarded by `selectedIndex >= 0` which is already enforced).
5. Add a keydown listener to `EntityView.vue` for:
   - `e` → navigate to edit form (fixes a latent bug: the shortcut is advertised
     in `KeyboardShortcutsModal.vue` but no handler exists)
   - `Delete` / `Backspace` → open ConfirmModal
   - Guarded by `isInputFocused()` and "no other modal open" check.
6. Add `<kbd>Del</kbd>` hint on the Delete button in `EntityView.vue` (matches
   existing `<kbd>E</kbd>` on Edit).
7. Update `KeyboardShortcutsModal.vue` to add `Delete` under **List View** and
   **Entity Detail** groups.
8. Tests: unit for `ConfirmModal`, update `useListKeyboard.test.ts` for
   Backspace, add `EntityView` keydown test, update e2e list-delete spec.

**Out of scope:**

- Bulk delete / multi-select.
- Undo / soft-delete.
- Backend delete semantics.
- Generalizing `EntityView`'s keyboard handling into a composable (keep local).

**Acceptance Criteria:**

1. **AC1 — List view uses styled modal.** Deleting a row from the list opens the
   same `ConfirmModal` used by the detail view. No `window.confirm` appears.
   *Test:* e2e spec navigates to a list, clicks the delete button on a row,
   asserts `.modal-overlay` is visible and `window.confirm` is never triggered
   (no `page.on('dialog')` needed).

2. **AC2 — List Delete/Backspace shortcut.** With a row selected (via `j`/`k`),
   pressing `Delete` or `Backspace` opens the confirm modal targeting that row.
   *Test:* `useListKeyboard.test.ts` dispatches both keys and asserts `onDelete`
   fires with the selected index; component test asserts modal opens.

3. **AC3 — Detail view Delete shortcut.** Pressing `Delete` or `Backspace` on
   the detail view opens the confirm modal.
   *Test:* `EntityView` unit test mounts the view, dispatches a keydown, asserts
   modal state is `open`.

4. **AC4 — Detail view `e` shortcut.** Pressing `e` on the detail view navigates
   to the edit form. Fixes a latent bug where the shortcut was documented but
   not implemented.
   *Test:* unit test asserts `router.push('/form/...')` is called.

5. **AC5 — Busy state.** While the delete request is in flight, both modal
   buttons are disabled and the confirm button shows a loading state.
   *Test:* `ConfirmModal.test.ts` asserts `:disabled` on both buttons when
   `busy=true`.

6. **AC6 — Escape and overlay close.** Escape closes the modal without
   deleting; clicking outside the modal closes it.
   *Test:* `ConfirmModal.test.ts` asserts `cancel` emit on Escape keydown and
   overlay click.

7. **AC7 — Discoverability.** The shortcuts modal (`?`) lists `Delete` under
   both List View and Entity Detail.
   *Test:* `StatusBar.test.ts` or a new `KeyboardShortcutsModal.test.ts`
   snapshots the items.

8. **AC8 — Guards.** Shortcuts do not fire when focus is in an input or when
   another modal is already open.
   *Test:* `useListKeyboard.test.ts` and new EntityView test assert no-op when
   `isInputFocused()` returns true or `.modal-overlay` is in the DOM.

9. **AC9 — No `window.confirm` remains.** A grep for `window.confirm` in
   `frontend/src/` returns zero hits.
   *Test:* ESLint `no-restricted-globals` rule or a repo grep in CI; at minimum
   a verification step in the implementation checklist.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Existing inline modal** at `frontend/src/components/entity/EntityDetail.vue`
  lines ~389-413: an overlay + modal structure with Cancel/Delete buttons, busy
  state, overlay-click-to-close. This is the proven pattern to extract.
  Note: file is named `EntityDetail.vue` but the grep earlier hit
  `EntityView.vue` — the delete state lives in `EntityDetail.vue` as a child
  component. Confirm exact file during implementation.
- **`useListKeyboard.ts:89-95`** already handles the `Delete` key and calls
  `onDelete(selectedIndex)`. `EntityList.vue:42-71` wires every callback
  *except* `onDelete` — adding it is a 5-line change. The composable
  intentionally excludes `Backspace` today (commented constraint). Extending it
  requires updating both the handler and the test.
- **`useKeyboardShortcuts.ts`** is the global handler and already owns Escape
  for closing the shortcuts modal. New per-view handlers must not conflict.
  Strategy: EntityView handles its own `e` / `Delete` only when no input is
  focused and no `.modal-overlay` is present, matching `useListKeyboard`'s
  existing guard (line 52).
- **`KeyboardShortcutsModal.vue:15-54`** is a computed list of shortcuts —
  adding items is purely declarative, no handler wiring.
- **Prior art: BUG-010** fixed a related keyboard shortcut bug and added
  `keyboard-shortcut-tests` as an automated measure. Worth linking the new
  component tests back to that measure via `adds-measure`.
- **No dialog/confirm library** in `frontend/package.json` beyond native DOM.
  Rolling our own ConfirmModal matches the existing custom-modal pattern used
  elsewhere (e.g., `HelpModal.vue`, `InlineCreateModal.vue`, `LinkExistingModal.vue`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Create `frontend/src/components/ui/ConfirmModal.vue`**
   - Props:
     - `open: boolean`
     - `title: string`
     - `message?: string` (slot fallback for richer content)
     - `confirmLabel?: string` (default `"Confirm"`)
     - `cancelLabel?: string` (default `"Cancel"`)
     - `busy?: boolean` (disables both buttons, shows loading label)
     - `danger?: boolean` (applies `btn-danger` to confirm button)
   - Emits: `confirm`, `cancel`
   - Default slot for custom body content; named `default` slot overrides `message`.
   - On `open` becoming true: `nextTick` then focus the Cancel button
     (safer default, mirrors `window.confirm` convention).
   - Escape key: listens only while `open` is true, emits `cancel`, calls
     `e.stopPropagation()` so the global Escape handler doesn't also fire.
   - Overlay click (click.self) emits `cancel`.
   - Teleport to body (matches `KeyboardShortcutsModal.vue` pattern).
   - Reuse existing `.modal-overlay` / `.modal` / `.btn` / `.btn-danger` /
     `.btn-secondary` styles from `App.vue` — no new CSS.

2. **Update `frontend/src/components/entity/EntityDetail.vue`**
   - Replace the inline `<div v-if="showDeleteConfirm">` modal (lines ~389-413)
     with `<ConfirmModal :open="showDeleteConfirm" ... />`.
   - Wire `@confirm="deleteEntity"` and `@cancel="showDeleteConfirm = false"`.
   - Pass `busy="deleting"`, `danger`, title `"Delete Entity?"`, message
     template with entity ID.

3. **Add keydown to `EntityView.vue` (or the detail component that owns state)**
   - `onMounted` → register `keydown` on `document`; `onBeforeUnmount` → remove.
   - Handler:
     - Return early if `isInputFocused()`.
     - Return early if `document.querySelector('.modal-overlay, .shortcuts-overlay')`.
     - `e.key === 'e'` → `router.push('/form/<edit_form>/<id>')` (mirror
       `EntityList.vue:50-55` logic; read edit form id from schema store).
     - `e.key === 'Delete' || e.key === 'Backspace'` → set
       `showDeleteConfirm = true`, `preventDefault()` to stop browser back nav.
   - Add `<kbd>Del</kbd>` hint after the Delete button text.

4. **Extend `useListKeyboard.ts`**
   - Add `Backspace` to the existing `Delete` case; guard by
     `selectedIndex >= 0 && options.onDelete` (already present).
   - `preventDefault()` on both keys to prevent browser back nav when a row is
     selected.

5. **Wire `onDelete` in `EntityList.vue`**
   - Add `pendingDelete: Ref<Entity | null>` state.
   - `onDelete: (index) => { pendingDelete.value = entities.value[index] }`
   - Replace `handleDelete`'s `window.confirm` with opening the modal; on
     confirm, run the existing delete API call.
   - Render `<ConfirmModal :open="!!pendingDelete" ... />` at the root of the
     component template.

6. **Update `KeyboardShortcutsModal.vue:34-46`**
   - List View group: add `{ keys: 'Delete', description: 'Delete selected entity' }`.
   - Entity Detail group: add `{ keys: 'Delete', description: 'Delete entity' }`.

7. **Tests**
   - **New** `frontend/src/components/ui/ConfirmModal.test.ts`: render open/closed,
     emit confirm/cancel, busy disables buttons, Escape emits cancel, overlay
     click emits cancel, default focus lands on Cancel button.
   - **Update** `frontend/src/composables/useListKeyboard.test.ts`: dispatch
     `Backspace` when row selected → `onDelete` called; Backspace with no
     selection → no-op.
   - **New** `EntityView` / `EntityDetail` keyboard test: `e` triggers
     navigation; `Delete` opens the confirm modal; input-focused → no-op.
   - **New** `KeyboardShortcutsModal.test.ts` (if absent): asserts Delete rows
     exist in List View and Entity Detail groups.
   - **Update** `frontend/e2e/` list-delete spec: remove any
     `page.on('dialog')` plumbing; click delete, assert modal appears, click
     Confirm, assert row gone.
   - **Update** `EntityList.vue` unit test if one exists (haven't verified yet).

**Alternatives considered:**

- **Generic `dialog` HTML element instead of custom modal**: native `<dialog>`
  has good accessibility defaults but requires polyfill considerations for
  older webviews and doesn't match the existing modal CSS language. Rejected —
  inconsistent with rest of app.
- **Pinia store for global confirm state** (`uiStore.confirm(...)` returning a
  promise): nicer call-site ergonomics but adds global state and one more
  store responsibility. Rejected — two call sites don't justify the indirection.
- **Move delete logic into a composable `useConfirmDelete`**: premature
  abstraction for two call sites. Rejected per CLAUDE.md guidance.
- **Keep `window.confirm` and just wire the keyboard shortcut**: rejected per
  user decision — the inconsistency is the whole point of the ticket.

**Files to modify:**

- `frontend/src/components/ui/ConfirmModal.vue` — new
- `frontend/src/components/ui/ConfirmModal.test.ts` — new
- `frontend/src/components/entity/EntityDetail.vue` — replace inline modal, add keydown + kbd hint
- `frontend/src/components/lists/EntityList.vue` — use ConfirmModal, wire onDelete, drop window.confirm
- `frontend/src/composables/useListKeyboard.ts` — accept Backspace
- `frontend/src/composables/useListKeyboard.test.ts` — cover Backspace
- `frontend/src/components/ui/KeyboardShortcutsModal.vue` — add Delete rows
- `frontend/e2e/<list delete spec>.ts` — update assertions (file TBD)

**Dependencies:** none new. Uses existing Vue 3, Pinia, vue-router, vitest,
Playwright.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Keyboard events**: OS-level `KeyboardEvent`. `isInputFocused()` guard
  prevents shortcuts firing while typing. `e.preventDefault()` on `Delete` /
  `Backspace` prevents browser back-nav hijack.
- **Entity ID passed to delete API**: same entity already fetched via typed
  API; no new input surface. Existing `entitiesStore.remove(type, id)` handles
  encoding.
- **Modal message**: displays entity ID. Vue template interpolation escapes
  HTML by default; no `v-html` used. Safe against stored XSS in entity IDs
  (which are also validated server-side).

**Security-Sensitive Operations:**

- **Destructive delete**: already required confirmation. The change makes the
  confirmation *more* deliberate (custom modal with explicit Cancel focus)
  rather than less. Backspace triggering delete is a new risk surface —
  mitigated by (a) requires a row to be already selected via j/k, (b) opens a
  confirm modal rather than deleting immediately, (c) `preventDefault()` stops
  the browser back-nav side effect.

**Error handling:** existing `uiStore.error('Failed to delete entity')` flow is
preserved. No sensitive info leaked.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Scenario | Test Type | Location |
|----|----------|-----------|----------|
| AC1 | Delete from list opens custom modal, no window.confirm | e2e | `frontend/e2e/list-delete.spec.ts` (verify name) |
| AC2 | Delete + Backspace on selected row fire onDelete | unit | `useListKeyboard.test.ts` |
| AC3 | Delete / Backspace on detail view opens modal | unit | `EntityDetail.test.ts` (new) |
| AC4 | `e` on detail view navigates to edit form | unit | `EntityDetail.test.ts` (new) |
| AC5 | busy=true disables both buttons, shows loading label | unit | `ConfirmModal.test.ts` (new) |
| AC6 | Escape emits cancel; overlay click emits cancel | unit | `ConfirmModal.test.ts` |
| AC7 | Shortcuts modal lists Delete under both groups | unit | `KeyboardShortcutsModal.test.ts` (new) |
| AC8 | No shortcut fires when input focused or modal open | unit | `useListKeyboard.test.ts` + EntityDetail test |
| AC9 | No `window.confirm` in `frontend/src/` | grep / CI | implementation checklist verification step |

**Edge Cases:**

- **No row selected + Delete pressed**: no-op (guarded by `selectedIndex >= 0`).
- **Modal already open + Delete pressed again**: no-op (guarded by
  `.modal-overlay` query).
- **Input focused inside a modal**: `isInputFocused()` returns true, no-op.
- **Delete request fails**: `uiStore.error` fires, modal stays open with busy
  cleared so the user can retry or cancel.
- **Entity fetched then deleted from under the user via SSE invalidation**:
  existing race — not worsened. `entitiesStore.remove` will 404, error toast
  fires.
- **Browser back-nav on Mac Backspace**: `preventDefault()` on the keydown when
  the shortcut fires; when it does *not* fire (no selection, input focused,
  modal open) Backspace behaves normally.
- **Keydown during route transition**: `onBeforeUnmount` removes the listener;
  no dangling handler.

**Negative Tests:**

- Dispatching `Delete` with no selection → `onDelete` not called.
- Dispatching `e` while `input` is focused → no navigation.
- `ConfirmModal` with `busy=true` → clicking buttons does not emit.
- Opening `ConfirmModal` while another modal is already open (edge case) →
  should still work but verify no double Escape handling.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Backspace hijacking browser back-nav unexpectedly**: Mitigated by
   requiring a selected row and calling `preventDefault()` only on the guarded
   path.
2. **Escape double-handling between `ConfirmModal` and global
   `useKeyboardShortcuts`**: Mitigated by `e.stopPropagation()` in the modal's
   Escape listener, and by the modal listener only being active while open.
3. **E2E test brittleness**: the current list-delete spec likely uses
   `page.on('dialog')`. Rewriting it is mandatory, not optional — flag in
   implementation checklist so it's not forgotten.
4. **File/component name uncertainty**: grep hit `EntityDetail.vue` for the
   delete modal state but `EntityView.vue` was also searched — need to verify
   which file owns the state at implementation start. Low risk, pure
   discovery.
5. **Coverage ratchet**: new files must ship with tests; frontend has 100%
   statement coverage via `FEAT-wzwp`. Plan includes tests for every new file.

**Effort:** `s` (small) — ~150 LOC production + ~200 LOC tests. One
self-contained PR.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — In-app documentation (`KeyboardShortcutsModal`) is the user-facing
      docs and is updated as part of the scope. No external docs, guide, or
      README references exist for the list-view delete flow or keyboard
      shortcuts beyond what the in-app modal surfaces.
- [x] ~~User guide / reference docs~~ (N/A: no external user guide exists for data-entry UI)
- [x] ~~CLI help text (if commands changed)~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md (if new patterns)~~ (N/A: no new repo-wide patterns; modalStack is a small local composable)
- [x] ~~README.md (if project-level changes)~~ (N/A: no project-level changes)
- [x] ~~API docs (if applicable)~~ (N/A: no API changes)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Skipped per user instruction ("hookup shortcuts")
— scope is small, self-contained, and follows existing patterns. No
architectural decisions that warrant formal review.

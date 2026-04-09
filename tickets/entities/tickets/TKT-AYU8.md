---
id: TKT-AYU8
type: ticket
title: Unify delete confirmation on custom modal and wire Delete shortcut in list & detail views
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

Delete confirmation is inconsistent across the data-entry UI:

- **Detail view (`EntityView.vue`)**: uses a styled custom modal (`showDeleteConfirm`), consistent with app theme, shows entity ID, disables buttons while the request is in flight.
- **List view (`EntityList.vue`)**: uses `window.confirm()` — native-styled, blocks the event loop, can't show a loading state, not stylable, inconsistent with the rest of the app.

Additionally, keyboard shortcuts for delete are not wired:

- `useListKeyboard` composable *supports* `onDelete` (triggers on `Delete` key) but `EntityList.vue` never passes an `onDelete` handler — so `Delete` key in a list does nothing.
- `EntityView.vue` has no keyboard listener at all. The modal claims `E` edits the entity (per `KeyboardShortcutsModal.vue`), but there's no actual keydown handler in `EntityView.vue` — likely a pre-existing bug to verify.
- No shortcut exists to delete from the detail view.

## Scope

**In scope:**

1. Extract the detail-view delete modal into a reusable `ConfirmModal.vue` (or `DeleteConfirmModal.vue`) component in `frontend/src/components/ui/`.
   - Props: `open`, `title`, `message`, `confirmLabel`, `busy`, `danger`.
   - Emits: `confirm`, `cancel`.
   - Default focus on Cancel button (safer default, matches `window.confirm` convention).
   - Closes on Escape / overlay click.
2. Use the new modal in `EntityList.vue` in place of `window.confirm()`.
3. Wire `onDelete` in `EntityList.vue`'s `useListKeyboard` call to open the modal for the selected row.
4. Add a keydown listener in `EntityView.vue` for:
   - `e` → edit (verify/fix the claim in `KeyboardShortcutsModal.vue`)
   - `Delete` → open delete confirm modal
   - Must respect `isInputFocused()` and not fire when any modal is open.
5. Add `<kbd>Del</kbd>` hint on the Delete button in `EntityView.vue` (matching the existing `<kbd>E</kbd>` on Edit).
6. Update `KeyboardShortcutsModal.vue`:
   - Add `Delete` → "Delete selected entity" under **List View**.
   - Add `Delete` → "Delete entity" under **Entity Detail**.
7. Tests:
   - Unit test for `ConfirmModal.vue` (open/close, emit, busy disables buttons).
   - Update `useListKeyboard.test.ts` — already tests `Delete` key dispatch.
   - Add keyboard test for `EntityView.vue` (e + Delete).
   - Update/extend e2e tests to assert list delete uses the custom modal (no `page.on('dialog')` shim).

**Out of scope:**

- Bulk delete / multi-select.
- Undo / soft-delete.
- Changing backend delete semantics.
- Moving Edit shortcut handling into a shared composable (keep EntityView local for now).

## Acceptance criteria

1. Deleting an entity from a list opens the same styled modal as the detail view (no `window.confirm`).
2. Pressing `Delete` (or `Backspace`? — decide during planning) on a selected row in list view opens the confirm modal for that row.
3. Pressing `Delete` on the detail view opens the confirm modal.
4. Pressing `e` on the detail view navigates to the edit form.
5. Both delete flows disable Confirm/Cancel while the request is in flight and show a loading state.
6. Escape and overlay click close the modal without deleting.
7. The keyboard shortcuts modal (`?`) lists the new shortcuts under their respective groups.
8. Shortcuts do not fire when focus is in an input or when another modal is already open.
9. No use of `window.confirm` remains in `frontend/src/`.

## Notes / risks

- `Backspace` as a delete shortcut is convenient on Mac (where there is no dedicated `Delete` key), but risky because it's also the browser's "back" shortcut when focus is not in an input. Current `useListKeyboard` explicitly restricts to `Delete` only — planning should revisit this explicitly.
- The `e` shortcut in the detail view being advertised but not implemented suggests a latent bug — confirm and either fix or file separately.

---
id: TKT-D3HY
type: ticket
title: Add keyboard delete shortcut to lists and entity detail (with confirmation)
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Allow users to delete entities via keyboard from the data-entry web app, in two
contexts:

1. **List/table view**: With a row selected (j/k navigation), pressing `Delete` opens a confirmation modal. On confirm, the entity is deleted and the list refreshes.
2. **Entity detail view**: Pressing `Delete` opens the existing delete confirmation modal. On confirm, the existing delete flow runs.

A confirmation modal is **always** shown — no immediate destructive action.

## Why

Power-users navigate the data-entry app heavily by keyboard (j/k, e, n, /, p/n,
Esc). Currently, deleting an entity requires reaching for the mouse to click the
Delete button on the entity detail page, and there is no way at all to delete
from the list view.

## Existing infrastructure

- `useListKeyboard` already exposes an `onDelete?` callback for the `Delete` key, but `EntityList.vue` does not currently pass it.
- `EntityDetail.vue` already has a `showDeleteConfirm` modal and `deleteEntity()` function — we just need to wire a keydown handler.
- The delete endpoint and post-delete navigation (`backTargetAfterDelete()`) already work correctly.

## Acceptance criteria

1. In a list view, with a row selected, pressing `Delete` opens a confirmation modal showing the entity ID. Confirming deletes the entity, the list refreshes, and a success toast is shown.
2. In the entity detail view, pressing `Delete` opens the existing delete confirmation modal. Confirming deletes and navigates per `backTargetAfterDelete()`.
3. Cancelling the modal (Esc, Cancel button, or click outside) leaves the entity untouched.
4. The shortcut does not fire while an input is focused or a modal is already open.
5. The `Delete` shortcut is documented in `KeyboardShortcutsModal.vue`.

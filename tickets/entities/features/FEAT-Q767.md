---
id: FEAT-Q767
type: feature
title: Keyboard shortcuts and power-user navigation in data-entry UI
summary: 'Consistent keyboard-driven workflow across list, detail, and form views in the data-entry Vue SPA: j/k navigation, g-prefix routing, ? help modal, e=edit, Delete=delete, Cmd+Enter save, Escape cancel.'
description: Give data-entry users a fast, consistent keyboard-driven workflow across list, detail, and form views so they can navigate, open, edit, create, and delete entities without the mouse. Covers global shortcuts (? help, / search, g-prefix navigation), list-scoped j/k navigation, Enter/o open, e edit, n create, Delete delete, h/l pagination, detail-view e/Delete, and form Cmd+Enter save / Esc cancel. Includes an in-app shortcuts modal kept in sync with the handlers.
priority: medium
status: in-progress
---

## Goal

Give data-entry users a fast, consistent keyboard-driven workflow across all
core screens (list, detail, form) so they can operate the app without reaching
for the mouse.

## Scope

- Global shortcuts (`?` help, `/` search, `g`-prefix navigation).
- List view: `j`/`k`/arrows, `Enter`/`o` open, `e` edit, `n` create, `Delete` delete, `h`/`l` pagination.
- Detail view: `e` edit, `Delete` delete.
- Form: `Cmd/Ctrl+Enter` save, `Esc` cancel.
- Discoverability: in-app shortcuts modal (`KeyboardShortcutsModal.vue`) kept in sync with actual handlers.
- Shortcuts must not fire while focus is in an input or when a modal is open.

## Implementation anchors

- `frontend/src/composables/useKeyboardShortcuts.ts` — global handler + g-prefix state machine.
- `frontend/src/composables/useListKeyboard.ts` — list-scoped handler.
- `frontend/src/components/ui/KeyboardShortcutsModal.vue` — user-facing documentation of shortcuts.
- `frontend/src/utils/dom.ts` — `isInputFocused()` guard shared by all handlers.

## Out of scope

- Fully customizable keybindings.
- Vim-mode text editing inside inputs.
- Command palette (`Cmd+K`) — reserved but separate future work.

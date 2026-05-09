---
id: TKT-77JD4
type: ticket
title: Quick-search/jump command palette for data-entry UI
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

Navigating between entities in the data-entry SPA today requires going back to a
list view, possibly switching entity type via the sidebar, and either scrolling
or using the per-list search. For users who already know the title or ID of the
entity they want, this is unnecessary friction.

VS Code, Linear, Notion, GitHub, and most modern editors offer a command-palette
/ quick-open modal that lets you type a fuzzy query and jump anywhere with the
keyboard. `FEAT-Q767` (keyboard shortcuts and power-user navigation) explicitly
listed `Cmd+K` command palette as out-of-scope future work; this ticket delivers
it.

## Goal

Let a power user press a single keyboard shortcut from anywhere in the
data-entry UI, type a few characters of an entity title or ID, and jump straight
to that entity's detail view — without using the mouse.

## Scope

In scope:

- A modal palette opened via a global keyboard shortcut (`Cmd+K` / `Ctrl+K`).
- A search input that queries entities by title, ID, and type.
- Results list with keyboard navigation (`↑`/`↓`) and `Enter` to open.
- Click-to-select for mouse users.
- `Esc` closes the modal; clicking the backdrop closes it.
- Reachable from any view (list, detail, form) and respects modal-open guards used by `useKeyboardShortcuts`.
- Display: each result shows entity title, ID, and entity-type label.
- Empty state: when query is empty, show a helpful hint. When query has no matches, show "No matches".

Out of scope:

- Running commands / actions (e.g. "create new ticket", "toggle theme"). Reserve for follow-up.
- Recently-visited / pinned entries.
- Search across views, dashboards, or other non-entity targets.
- Server-side ranking changes (use existing search endpoint).
- Customizing the keybinding.

## Acceptance criteria

- AC1: From any data-entry route, pressing `Cmd+K` (macOS) or `Ctrl+K` (other) opens the palette modal.
- AC2: The palette opens even when focus is in a text input/textarea (the shortcut explicitly bypasses `isInputFocused` because users expect Cmd+K from anywhere).
- AC3: Typing into the input shows matching entities ranked by the existing search backend; updates are debounced to avoid request-per-keystroke.
- AC4: `↑` / `↓` move the highlighted result; the highlight wraps at the ends.
- AC5: `Enter` navigates to the highlighted entity's detail route and closes the modal.
- AC6: Clicking a result navigates and closes the modal.
- AC7: `Esc` closes the modal without navigating; backdrop click closes it; focus returns to the previously focused element.
- AC8: Re-opening the palette starts with an empty query and the first result (if any) highlighted.
- AC9: The shortcut is documented in `KeyboardShortcutsModal.vue`.
- AC10: When focus is inside the palette input, the existing global `useKeyboardShortcuts` handlers (e.g. `g`-prefix nav) are suppressed so typing letters does not trigger navigation.

## Non-functional

- Initial open and result rendering perceptibly under 100ms for typical projects.
- Keyboard-only operability (no mouse needed for any flow).
- Accessibility: modal traps focus (use shared `useFocusTrap` if available), aria-roles for combobox/listbox, results announced.

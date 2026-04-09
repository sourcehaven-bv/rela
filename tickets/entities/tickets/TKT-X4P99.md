---
id: TKT-X4P99
type: ticket
title: Add shared useFocusTrap composable and apply to all data-entry modals
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

None of the data-entry modals (`ConfirmModal`, `HelpModal`,
`KeyboardShortcutsModal`, `CommandModal`, `InlineCreateModal`,
`LinkExistingModal`) implement a focus trap. Tab can escape into the background
page, which is poor accessibility and a minor usability bug for keyboard users.

## Scope

- Add `frontend/src/composables/useFocusTrap.ts` — trap Tab/Shift+Tab inside
a given root element while active.
- Wire into all six modals listed above.
- Verify with axe or manual keyboard testing.

## Out of scope

- Replacing the modal stack registry (already in place from TKT-AYU8).
- Rewriting the modal component hierarchy into a single base class.

---
id: TKT-60E9G
type: ticket
title: Replace remaining window.confirm() calls in data-entry UI with ConfirmModal
kind: refactor
priority: low
effort: xs
status: backlog
---

## Problem

TKT-AYU8 introduced a shared `ConfirmModal` component and replaced
`window.confirm` in the delete flow. Two other `window.confirm` calls remain in
`frontend/src/`:

1. **`CommandModal.vue:21`** — `if (cmd.confirm && !confirm(cmd.confirm))`.
Used to confirm before running a command (Lua script). Commands can be
destructive (edit files, delete data), so a styled modal is arguably more
important here than for entity delete.

2. **`DynamicForm.vue:481`** — unsaved-changes prompt when navigating away
from an in-progress form.

Both should be migrated to `ConfirmModal` for consistency. Neither was in scope
for TKT-AYU8, which was specifically about the delete button UX.

## Notes

- `DynamicForm` unsaved-changes prompt is trickier because it's tied to route
navigation guards, not a simple boolean. May need a promise-based confirm API
(see also leverage idea L2 from the TKT-AYU8 review).
- `CommandModal` use is a straightforward sync boolean that can be
refactored directly.

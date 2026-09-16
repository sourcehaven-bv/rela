---
id: RR-H2LHT7
type: review-response
title: The wrap path dispatched twice and bypassed the dispatch it was given
finding: 'On a bare paragraph the command called `wrapInList(...)(state, dispatch, view)` and then called `view.dispatch(...)` directly, ignoring the `dispatch` argument it was handed. That violates the ProseMirror contract that a command routes every change through the dispatch it was given, so anything wrapping the command (a transaction filter, a collab plugin, an appendTransaction inspecting dispatch order) would see a change it never authorized. The `view &&` guard followed by `view!` was also telling: with no view the command returned true having wrapped the paragraph in a plain bullet list and NOT marked it as a task - reporting success for a job it half did. Undo granularity happened to be correct only because prosemirror-history''s newGroupDelay merges adjacent dispatches, which is default timing doing us a favour rather than a guarantee.'
severity: significant
resolution: The wrap path now hands `wrapInList` a capturing dispatch, applies `setNodeMarkup` for every resulting item to that same transaction, and dispatches ONCE through the caller's `dispatch`. The `view` argument is no longer used at all, so the half-done outcome is unreachable. A test asserts a single Ctrl-Z fully reverts `- [ ] loose text` to `loose text` rather than stranding the user on the intermediate `- loose text`, so the undo behaviour is now pinned rather than incidental.
status: addressed
---

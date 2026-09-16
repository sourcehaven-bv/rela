---
id: RR-465KMS
type: review-response
title: The task-list command converted only one item of a multi-item selection
finding: '`findListItem` walked up from the selection head only and returned a single item, so the command marked exactly one node. Selecting across two list items and pressing the button produced `- [ ] alpha\n- beta` - half the selection converted, with `aria-disabled: false` giving no signal the operation was partial. On the wrap path it was worse: two selected paragraphs became a two-item list with only the first marked, so the user saw one checkbox and one bullet. Toggling off was equally partial. Every neighbouring block command (Bullet list, Numbered list, Quote) applies across a selection; this one silently did not.'
severity: critical
resolution: 'Replaced `findListItem` with `selectedListItems`, which returns the single enclosing item for a collapsed cursor and every touched `list_item` for a range, via `nodesBetween` without descending into nested lists (so selecting a parent does not also toggle its children). All items are marked in ONE transaction. A mixed selection turns ON - if any item is not yet a task, the press makes them all tasks, and a ticked item keeps its tick - because a single press that converted some and reverted others would leave the list exactly as inconsistent as before. Four tests cover it: all-bullets, all-paragraphs (wrap path), all-tasks (revert), and mixed.'
status: addressed
---

## A test bug found on the way

The first version of the selection test used `TextSelection.create(doc, 1,
doc.content.size - 1)`. The `trailing` plugin keeps an empty paragraph at the
end of every document, so that range ends in a node with no text positions and
ProseMirror collapses the whole selection into it — the command then saw a
cursor in an empty paragraph and wrapped *that*, producing a stray `*` in the
output.

The helper now selects from inside the first text block to inside the last one
that actually holds text, which is what dragging across the list does. The
reasoning is recorded on the helper so the next person does not reach for the
obvious-looking range.

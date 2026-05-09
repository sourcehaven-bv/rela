---
id: RR-W7S4F
type: review-response
title: List item shape is heterogeneous (string OR table); walker must handle both
finding: 'extractListItems at markdown.go:673 emits items as either a Lua string (plain item) or a Lua table {task, checked, text} (task item). Plan says ''strings or tables with text field'' but doesn''t pin down the asymmetric mutation: replace the int-keyed slot when it''s a string; mutate the text field only (preserving task/checked) when it''s a table.'
severity: minor
resolution: 'Plan Approach pins the asymmetric mutation: LString item → replace int-keyed slot; LTable item → mutate text only, preserve task/checked. AC20 covers plain + task list items in the same fixture.'
status: addressed
---

# Finding

`extractListItems` (`markdown.go:673`) emits list items as **either** a Lua
string (plain item) **or** a Lua table `{task=true, checked=bool, text="..."}`
(task-list item). The plan says "items are strings or tables with a text field"
but doesn't pin down the asymmetric mutation. Easy to implement one path and
forget the other.

# Resolution

Add to Approach as explicit:

> **List-item handling.** For each item in a list node:
>
> - `LString` item: replace the int-keyed slot with a new `LString` (in
>   the deep-copied list).
> - `LTable` item: mutate only the `text` field; preserve `task` and
>   `checked`.
> - Other types: skip (defensive).

Tests:

- Plain list with one ID: `- See TKT-1` → string slot rewritten.
- Task list with one ID: `- [x] See TKT-1` → `text` rewritten,
`task=true`/`checked=true` preserved.
- Mixed list with both kinds.

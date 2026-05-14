---
id: TKT-I5NO
type: ticket
title: Add internal-link picker button to the markdown editor toolbar
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

The data-entry markdown editor (`MarkdownEditor.vue`, EasyMDE) has a default
`link` toolbar button that inserts a generic markdown `[](url)` placeholder.
There is no fast way to insert a reference to another entity inside the body
content — authors have to switch tabs, find the entity, copy its ID, and
hand-type `` `TKT-XYZ` ``. With TKT-747O just shipped, those backticked IDs now
render as titled in-app links — so the natural next step is a dedicated toolbar
affordance to produce them.

## Proposal

Add a new custom toolbar button next to the existing `link     ` button (or
replacing it, TBD in planning) that opens an entity picker. The picker searches
the project's entities (using the existing `/_search     ` endpoint or a similar
in-process source) and, on selection, inserts `` `<id>` `` at the cursor
position. The renderer (TKT-747O) handles the title resolution and link
affordance on display.

## Acceptance criteria

1. A new toolbar button appears in `MarkdownEditor.vue     `'s EasyMDE toolbar
with a clear icon and tooltip ("Insert entity reference").
2. Clicking the button opens an entity-picker modal/popover scoped to the
project, searchable by ID and title.
3. Selecting an entity inserts `` `<id>` `` at the current cursor position
(or replaces the current selection).
4. The picker is keyboard-accessible: open via shortcut, focus the search,
arrow keys to navigate, Enter to select, Esc to dismiss.
5. The editor stays focused and the cursor lands immediately after the
inserted code span so the user can keep typing.
6. The inserted code span round-trips: viewing the saved entity in the
data-entry view renders it as a titled link via TKT-747O's resolver.
7. Unit tests cover the insertion logic (cursor position, selection
replacement, no entity selected).
8. Playwright e2e covers the happy path: open form, click button, pick
entity, save, verify the rendered link on the detail page.

## Out of scope

- A new server-side search endpoint — reuse `/api/v1/_search     `.
- The reverse direction (auto-completion as you type `     ` `) — that's a
bigger feature; this is the deliberate-insertion path.
- Wiki-style `[[...]]     ` syntax (IDEA-011).

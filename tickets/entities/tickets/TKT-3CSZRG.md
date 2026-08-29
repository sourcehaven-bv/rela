---
id: TKT-3CSZRG
type: ticket
title: Ctrl/Cmd-click (and middle-click) should open data-entry rows and cards in a new browser tab
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

Navigable surfaces in the data-entry SPA are plain `<div>` / `<tr>` elements
with an `@click` handler calling `router.push`. Because they are not anchors,
ctrl/cmd-click, middle-click, shift-click and the browser's own "Open link in
new tab" context menu all fail — the row either navigates in the same tab or
ignores the click entirely. Users expect anything that looks like a link to be
openable in a background tab.

| Interaction | Expected | Actual |
|---|---|---|
| Cmd/Ctrl+click | open in background tab | navigates current tab |
| Middle-click | open in background tab | nothing |
| Shift+click | open in new window | navigates current tab |
| Right-click → Open in new tab | menu entry present | no menu entry |
| Hover | status-bar URL preview | nothing |

## Affected surfaces

- `components/lists/EntityList.vue` — desktop `<tr class="entity-row" @click="navigateToEntity(entity)">` and the mobile `<div class="mobile-card" @click>`
- `views/KanbanView.vue` — card `<div @click="openCard(entity)">` (two call sites)
- `components/calendar/CalendarEventChip.vue` — `@click="$emit('open', event)"`
- `components/forms/SidePanel.vue`, `components/forms/RelationCards.vue` — entry click handlers
- `components/common/IssuesTable.vue` — row click
- `views/SearchView.vue` — result click

`utils/entityRoute.ts` already exists precisely for this, and the comment at
`EntityList.vue:533` claims rows open "through a real `<a href>` on the row
markup elsewhere" — that is aspirational, not true. The helper currently only
computes a path that is then handed to `router.push`.

`composables/useDocumentClicks.ts` already gets this right for links inside
rendered document HTML: it bails out on modifier/middle clicks and lets the
browser take over. That is the model to generalize.

## Approach sketch

Render the navigable element as an anchor with a real `href`, and make the click
handler defer to the browser when the user signalled new-tab intent
(`event.metaKey || ctrlKey || shiftKey || altKey || button !== 0`). A shared
helper keeps the predicate in one place rather than repeating a five-way
modifier check at every call site.

Table rows cannot be wrapped in an anchor (invalid HTML inside `<tr>`), so the
row keeps its click handler but must honour modifier-clicks; the primary/title
cell additionally gets a real anchor so right-click and middle-click work.

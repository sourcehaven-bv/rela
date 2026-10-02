---
id: TKT-ITQ0HL
type: ticket
title: 'Pages v1: pages: in data-entry.yaml, /p/<page>/<tab> route, tab bar in the page header'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

First slice of FEAT-9F2S23.

### Config

```yaml
pages:
  tickets:
    label: Tickets
    icon: ticket
    tabs:
      - { id: board, kanban: ticket_board, label: Board }
      - { id: table, list: tickets, label: Table }
      - { id: timeline, gantt: roadmap, label: Timeline }

navigation:
  - page: tickets
```

- A tab is a navigation destination (list, kanban, gantt, calendar, dashboard,
document) plus an `id`, a `label` and an optional `icon`. It takes `permission:`
like a navigation entry.
- Tab ids follow a fixed pattern (like space ids) and are unique within the
page, since they appear in the URL.
- `page:` is a new navigation destination, so it works in spaces, in a
space's `home:`, with `icon:` and with `permission:`.

### Decisions (agreed 2026-09-27)

- **The tabs belong to the page, not to the navigation entry.** A URL reached
from a bookmark has no navigation entry to take tabs from, and one list can sit
under several entries. The URL names the page, so the tab bar never depends on
how the user arrived.
- **URL:** `/p/<page>/<tab>`, under a space `/s/<space>/p/<page>/<tab>`. The
tab's own state (sort, filters, `?selected=`, `?world=`) stays in the query
string as on the standalone route; switching tabs drops it.
- `/p/<page>` redirects to the first tab the principal may see. An unknown tab
id redirects to that tab too; an unknown page id shows the usual "not in the
configuration" state.
- The standalone routes (`/list/<id>`, `/kanban/<id>`, ...) stay and render
without a tab bar.
- Tabs the principal may not see are left out. With one tab left the bar is
hidden. A page with no visible tab is left out of the sidebar, like an empty
list entry.
- No "+ Add" tab in v1 (`RlViewTabs` `show-add="false"`).

### Scope

- `dataentryconfig`: the `pages:` type and validation: ids, tab ids, unknown
list/kanban/gantt/calendar/document references, `page:` references from
navigation and spaces, permission warnings like TKT-E5EM3N. Error messages name
the page and the tab.
- Sidebar API and nav status: a `page:` entry is one destination. Its count
(if any) comes from the first tab.
- SPA: the `/p/:page/:tab?` route (and under `/s/:space/`) mounts the tab's
existing view with the page header's `RlViewTabs`. Tabs are links, so middle
click and back work. `panelMode` comes from the tab's view kind (timeline is
`overlay`). The sidebar highlights the page on any of its tabs.
- **Back links:** an entity page's Back button and a create form's Cancel go
to `/list/<id>` today (`EntityDetail.vue`, `useBackTarget.ts`,
`DynamicForm.vue`). When the user came from a page tab they must lead back to
`/p/<page>/<tab>`.
- Docs: a data-entry.md section on `pages:`.

## Out of scope

- Search and filters shared across tabs.
- Views scoped to one parent entity from the URL.
- User-created tabs.

## Acceptance criteria

- A config without `pages:` renders and routes exactly as today.
- `/p/tickets/table` shows the Tickets page with its tab bar and the Table tab
selected, whether reached from the sidebar, a bookmark or a reload.
- Switching tabs changes the URL; back returns to the previous tab.
- Opening an entity from a tab and pressing Back returns to that tab.
- A tab whose `permission:` the principal lacks is absent; the others remain.
- Invalid references in `pages:` fail config load with a message naming the
page, the tab and the key.

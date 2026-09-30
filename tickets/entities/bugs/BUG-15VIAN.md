---
id: BUG-15VIAN
type: bug
title: A nav entry with an unrecognised kind gets an empty href and renders as a dead row
description: 'tickets/data-entry.yaml declares `graph: true`, a nav kind NavigationEntry has no field for. It loads silently and is served with href: "", which as a RouterLink target matches every route.'
priority: low
status: backlog
---

## Description

`tickets/data-entry.yaml` line 1518 declares a nav entry as:

```yaml
  - label: "Graph"
    graph: true
```

`NavigationEntry` (`internal/dataentryconfig/config.go`) has **no `graph`
field**, and the `switch` in `internal/dataentry/views_handler.go` that assigns
`item.Href` has no matching case. The entry therefore loads without error and is
served as `{"label": "Graph", "href": ""}`.

Two defects, one in config and one in the loader:

1. rela's own dogfooding config names a nav kind that does not exist.
2. An unrecognised nav kind is accepted silently rather than refused at load.
An empty `Href` is a legitimate value for an `action:` entry (the code says so),
so nothing downstream can distinguish "button" from "broken".

## How it surfaced

The old sidebar template rendered items with `v-if="item.action"` /
`v-else-if="item.href"`, so an entry with neither was silently dropped and the
Graph row never appeared. Migrating to `RlSidebar` (TKT-ME8LEI) mapped it to a
`RouterLink` with `to: ''`, which vue-router resolves against the CURRENT
location. The row then matched every route and took the active highlight from
the row that should have had it, on every page including ones with no nav entry
at all.

Worked around in the frontend by `isNavigable` in
`frontend/src/components/common/sidebarNav.ts`, which drops an entry with
neither an action nor a non-empty href. Pinned by `sidebarNav.test.ts > drops an
entry with an empty href`.

## Suggested fix

Refuse the config at load. A nav entry matching no kind should be a load-time
error like other invalid config, which turns a silent dead row into a message
naming the file and key. Then either add a `graph:` kind or correct the tickets
project's entry.

The frontend guard should stay regardless: it is defence against malformed input
from any source, and the failure it prevents is silent.

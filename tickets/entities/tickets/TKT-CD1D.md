---
id: TKT-CD1D
type: ticket
title: Sync data-entry list filters with URL query params
kind: enhancement
priority: medium
effort: m
status: done
---

## Description\n\nWire up bidirectional sync between the data-entry list filter UI and the URL query string, so list views are bookmarkable, shareable, and deep-linkable from external tools (e.g. SwiftBar menu items).\n\n## Behavior\n\n**Read (URL → state):** On list mount and on URL change, parse `filter[prop]=value` and `filter[prop][op]=value` query params and apply them as the initial filter state. Filter controls (the ones the user can edit) are pre-filled from URL.\n\n**Write (state → URL):** When the user changes a filter control, update the URL query string without a full page reload (router.replace, no history spam).\n\n**Override semantics:**\n- URL params override `filter_controls` (user-facing filters) for the matching properties\n- Static `filters:` from `data-entry.yaml` are NEVER overridden — they remain locked, AND-combined with any URL filters\n- Removing a URL param falls back to the filter control's default value\n\n## Why\n\n- Bookmarkable filtered views (\"Open tasks due this week\")\n- Shareable links between collaborators\n- External tools (SwiftBar, command palette, scripts) can deep-link to filtered views\n- Browser back/forward navigates filter history naturally

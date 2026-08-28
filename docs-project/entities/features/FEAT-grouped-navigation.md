---
id: FEAT-grouped-navigation
type: feature
title: "Grouped Navigation"
status: published
summary: "Named sidebar groups for organizing navigation items in the data entry web app"
---

Navigation items in `data-entry.yaml` can be organized into named groups,
rendered as titled sections in the sidebar. The `collapsed` config flag is
kept for compatibility but the current SPA renders groups always expanded
(the old server-side collapse persistence has been removed). Nested groups
are rejected at validation time.

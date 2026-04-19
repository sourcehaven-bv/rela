---
id: FEAT-grouped-navigation
type: feature
title: "Grouped Navigation"
status: published
summary: "Collapsible sidebar groups for organizing navigation items in the data entry web app"
---

Navigation items in `data-entry.yaml` can be organized into named collapsible
groups. Collapsed state is persisted server-side, and groups auto-expand when
they contain the active page. Nested groups are rejected at validation time.

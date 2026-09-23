---
id: RR-WZ9GU7
type: review-response
title: Hot config reload leaves stale entries
finding: Sidebar loads once on mount (Sidebar.vue:103). A renamed scope yields 400 invalid_query_scope until reload.
severity: minor
resolution: Sidebar reloads /_sidebar on the refresh SSE event.
status: addressed
---

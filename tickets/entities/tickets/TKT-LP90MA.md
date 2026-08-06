---
id: TKT-LP90MA
type: ticket
title: Sidebar counts are fetched once on mount and never refresh
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

`Sidebar.vue` calls `loadSidebar()` in `onMounted` and never again — there is no
watcher on route changes and no `entity:changed` SSE subscription (contrast
`DocumentView.vue` / `DocumentsPanel.vue`, which both re-render on that event).

So the per-list and per-kanban counts shown in the menu are a snapshot from page
load. Create ten tickets and "Open Tickets" still shows the old number until a
full browser reload. The counts are correct when fetched — they run through the
ACL read scope (`sidebarCounts`, TKT-VMD8) — just stale.

## Notes for whoever picks this up

- `handleV1Sidebar` recomputes every count per request
(`countWithFilters`, `views_handler.go`), and with `filters:` configured it
loads the visible set into memory. A naive "refetch on every `entity:changed`"
would run that on every write in the system, for every connected client. See the
performance caveat in `docs/acl-security.md` § "Sidebar config-filter
performance caveat" (RR-REQW) — debounce, or refetch on navigation rather than
on the event stream.
- `countWithFilters` degrades backend errors to `0` with a `slog.Warn`. A
refresh that silently rewrites a count to 0 on a transient error is worse than a
stale count; consider distinguishing error from empty first.

## Origin

Noticed while surveying the sidebar for TKT-TXDK8U (nav filtering). Explicitly
split out by the user as a separate problem — TKT-TXDK8U does not touch counts.

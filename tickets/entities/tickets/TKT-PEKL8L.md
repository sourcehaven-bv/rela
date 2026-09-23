---
id: TKT-PEKL8L
type: ticket
title: Query-driven entity lists in sidebar navigation groups
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Operators want the sidebar to show live entity links, not only static entries.
Example: a "Projects" group that lists every `project` entity with `status ==
active`, each entry linking to that project's detail page.

Today every `navigation:` entry in `data-entry.yaml` is static (list, kanban,
document, view, and so on). This ticket adds a navigation entry kind that is
backed by a query. The server evaluates the query per principal, and the sidebar
renders one link per result.

## Constraints from history

- Sidebar counts were removed (TKT-VKM1E9) for three reasons: per-request cost that grew with the visible set, staleness, and an aggregate leak surface. This feature must answer all three.
- `/api/v1/_config` and the sidebar structure stay principal-independent (docs/acl-security.md, "Sidebar menu structure is principal-independent"). Query results are entity content, so they must come from a per-principal, ACL-gated path, as `/api/v1/_dashboard` does for dashboard cards.

## Acceptance criteria (draft, refined in planning)

- An operator can declare a query-backed navigation entry with an entity type, a filter, a sort order, and a result limit.
- The sidebar shows one link per matching entity that the principal may read. Hidden entities never appear and never affect what is shown.
- The result count is bounded by a configured limit with a hard server-side cap.
- Invalid config (unknown type, bad filter) fails at load, not at request time.
- Documented in the data-entry guide.

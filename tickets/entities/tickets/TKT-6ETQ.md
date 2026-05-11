---
id: TKT-6ETQ
type: ticket
title: Rename V1View{Add,Link}Info to V1SidePanel* now that view path no longer uses them
kind: refactor
priority: low
status: backlog
---

## Problem

After TKT-651W made entity views strictly read-only, `V1ViewAddInfo`,
`V1ViewLinkInfo`, and `V1ViewAddTarget` in `internal/dataentry/api_v1.go` are
used exclusively by `V1SidePanelSection`. The "View" prefix is a tenant of the
previous architecture and now lies about scope.

A future contributor could reach for `V1ViewAddInfo` while building a new
view-related response struct — the name suggests it's a generic view primitive —
and silently re-bleed mutation affordances back into the read-only view surface.

## Proposed change

- Rename `V1ViewAddInfo` → `V1SidePanelAddInfo`
- Rename `V1ViewLinkInfo` → `V1SidePanelLinkInfo`
- Rename `V1ViewAddTarget` → `V1SidePanelAddTarget`
- Move the type definitions from their current `V1View*` neighbourhood
to next to `V1SidePanelSection` so the read-only invariant is structurally
enforced (no add/link types in the view's neighborhood).

## Out of scope

- The wire shape stays identical (same JSON tags); only Go-side names move.
- Frontend types are already separately named (`SidePanelAddInfo` etc. in
`frontend/src/types/entity.ts`); no frontend change.

## Why deferred

TKT-651W focused on the read-only-view invariant. Renaming was deferred to keep
that ticket's diff scoped and reviewable.

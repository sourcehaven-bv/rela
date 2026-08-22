---
id: TKT-VKM1E9
type: ticket
title: Remove sidebar entity counts (badges, ACL-scoped counting path, docs)
kind: chore
priority: low
effort: s
status: review
---

## Problem

The sidebar showed an entity count badge next to each list and kanban nav entry.
The counts were low-value in practice — they answer a question ("how many
tickets exist right now?") that the list itself answers better the moment you
click it — and they carried real cost:

- `handleV1Sidebar` recomputed every count on every sidebar request. With
`filters:` configured, `countWithFilters` loaded the whole visible set into
memory and filtered it in Go; cost scaled with the principal's visible-set size
(the perf caveat recorded in `docs/acl-security.md`, RR-REQW).
- They were stale anyway: fetched once on mount, never refreshed
([[TKT-LP90MA]]). So the number you saw was a page-load snapshot.
- Counting is a read-side aggregate over ACL-gated rows, so it needed its own
gated code path (`sidebarCounts`, TKT-VMD8) with a standing leak surface — the
cardinality of hidden entities must never show through.

Removing the feature removes all three at once.

## What

Deleted:

- `sidebarCounts` (type + `listCount` / `kanbanCount` / `countWithFilters`) in
`internal/dataentry/views_handler.go` — ~120 lines including the GraphCount /
GraphQuery-then-filter counting and its error-degradation paths
- `Count *int` from `v1.SidebarItem` (`internal/apiwire/v1/responses.go`), so
`/api/v1/_sidebar` no longer emits `count`
- The two count-badge spans and `.nav-count` CSS in `Sidebar.vue`; `count?:
number` from the SPA `SidebarItem` type
- `acl_sidebar_test.go` in full (197 lines, count-only: four tests plus their
helpers) and `TestV1SidebarAppliesListFilters` /
`TestV1SidebarAppliesKanbanFilters`

`navEntryToSidebarItem` lost its `ctx` and `counts` parameters. Stale comments
corrected in `app.go` (claimed the sidebar "routes every count through the ACL
read scope", pinned by a now-deleted test) and `readgate.go` (cited sidebar
counts as a `ReadQuery` consumer).

**Kept**: nav `permission:` filtering and its tests, the icon tests, and the
whole read-gate machinery — `ReadQuery` still backs the list pipeline.

## Docs

Removed the count-badge description from `data-entry.md`, and the "Sidebar
counts go through the same gate" and sidebar config-filter performance sections
from `acl-security.md`; trimmed sidebar counts from the aggregate-gating lists
in `server-security.md`. Applied to both `docs/` and the
`docs-project/entities/guides/` sources so the mirrors stay in sync.

## Follow-up

[[TKT-LP90MA]] (stale sidebar counts) is obsoleted by this change — the feature
it wants to fix no longer exists.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./internal/dataentry/...` green
- `golangci-lint` 0 issues; `just arch-lint` OK; `just coverage-check` floors
pass (77.4% total)
- Frontend `typecheck` clean; 1824 unit tests pass

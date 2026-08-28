---
id: IMPL-JHMS30
type: implementation-checklist
title: 'Implementation: Remove sidebar entity counts (badges, ACL-scoped counting path, docs)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: pure deletion — no new code; the removals are enforced at compile time, and count-only tests were deleted rather than rewritten)
- [x] ~~Integration tests written~~ (N/A: no new flow; the surviving sidebar flow is already covered by `TestNavPermission_*` and `nav_icon_test.go`)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The `countWithFilters` error path that degraded backend failures to `0` with a
`slog.Warn` is gone along with the counting itself — one fewer place where a
real backend error rendered as a plausible-looking number. Nothing new swallows
errors; `slog` became unused in `views_handler.go` and was removed.

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

Tests were deleted, not written. `acl_sidebar_test.go` went in full (all four
tests asserted counts only; its helpers `seedSidebarWorld` / `sidebarPolicy` /
`installSidebarConfig` / `sidebarCountsByLabel` were confirmed unused elsewhere
before deleting). `TestV1SidebarAppliesListFilters` and
`TestV1SidebarAppliesKanbanFilters` likewise. `nav_icon_test.go` kept its
assertions and changed only the call signature.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Drove `handleV1Sidebar` directly with a seeded ticket, one configured list and
one nav entry, and inspected the raw response body:

```
status=200
{"app":{"name":"Test App","description":"Test Description"},
 "navigation":[{"items":[{"label":"All","href":"/list/all","icon":"list"}]}]}
```

- AC1 ✅ no `count` key in the payload despite a matching entity existing
(asserted on the raw JSON string, not a decoded struct, so a renamed field could
not slip past). Probe was temporary and removed.
- AC2 ✅ `npm run typecheck` clean with `count` gone from `SidebarItem`; 1824
frontend unit tests pass (113 files).
- AC3 ✅ `sidebarCounts` and its three methods no longer exist — verified by
`go build ./...` plus a grep returning no hits.
- AC4 ✅ label, href and icon still resolve (above); `go test
./internal/dataentry/...` green, including all `TestNavPermission_*` and the
icon tests.
- AC5 ✅ grep for `sidebar count` / `nav-count` / `listCount` / `kanbanCount` /
`sidebarCounts` across `docs/`, `docs-project/`, `internal/`, `frontend/src`
returns nothing (one straggler found this way and fixed: a stale `ReadQuery`
consumer comment in `readgate.go`).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the [[TKT-PZTP1L]] removal shape: delete the layer, then correct every
comment and doc that described it rather than leaving stale prose. No DRY
extraction applicable — the change only removes code. Security direction is
strictly safer: the hidden-cardinality leak surface is gone rather than
narrowed, and the shared read gate (`ReadQuery`, still used by
`scopedSortedEntities`) is untouched. Temporary verification probe deleted; no
debug code remains.

**Checks run:** `go build ./...`, `go vet ./...`, `go test
./internal/dataentry/... ./internal/apiwire/...` green; `golangci-lint` 0
issues; `just arch-lint` OK; `just coverage-check` floors pass (77.4% total);
frontend `typecheck` clean, 1824 unit tests pass.

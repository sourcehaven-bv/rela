---
id: IMPL-TZHZOE
type: implementation-checklist
title: 'Implementation: ACL read-side (PR 2/2): list endpoints + sidebar counts + pagination headers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Tests: `internal/acl/policy_test.go`
(`TestLoadPolicy_WriteWithoutRead_Rejected`, 7 cases incl. wildcard
combinations), `internal/dataentry/acl_list_test.go`
(AC1/AC2/AC3/AC4/AC5/AC9/AC10 + `TestACLList_QueryErrorMapping` for the
errACLListQuery → writeGateError routing),
`internal/dataentry/acl_sidebar_test.go` (AC6/AC7 + DenyAll-zero + NopACL-full
single-mode), `internal/dataentry/acl_list_regression_test.go` (AC11 list shape
+ free-text untouched under NopACL). Error handling: ACL GraphQuery failures
wrap `errACLListQuery` and route through `writeGateError` (500
`acl_query_failed` / 504 / silent-on-cancel) instead of mislabeling as
`search_failed`; sidebar counts degrade to 0 with `slog.Warn` (parity with the
old CountEntities error path).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reused PR-1 harness (`mustNewACL`, `gateCtxFor`, `principalCtx`); shared
`seedSidebarWorld`/`sidebarPolicy`/`installSidebarConfig` builders; recording
doubles (`recordingSearcher`, `orderRecordingStore`, `failingGraphQueryStore`)
for ordering/short-circuit asserts.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Live `rela-server` against a scratch project (10 tickets: 5 belongs-to PRJ-42
which alice is editor-of, 5 hidden; `inherit_roles_through: [belongs-to]`;
`--principal-header X-Rela-User`):

- alice `GET /api/v1/tickets?per_page=3&page=2&sort=title` → data `[TKT-V04, TKT-V05]`, meta `{total:5, page:2, per_page:3, has_more:false}`, `X-Total-Count: 5`, `Link` rel="last"→page=2, **no rel="next"**, `_actions.create: false`, `Cache-Control: no-cache, no-store, must-revalidate`, `Vary: X-Rela-User`. The hidden total 10 appears nowhere. (AC1/AC2/AC3/AC10)
- bob (no grants) → `data: []`, total 0, `X-Total-Count: 0`, `_actions.create: false`. (AC4)
- alice `?q=Hidden` → `[]` (search intersects post-ACL). (AC5/AC9)
- Sidebar alice: All Tickets=5, Open Tickets=3 (visible∩open); bob: 0/0. (AC6/AC7)
- Per-entity parity intact: hidden TKT-H01 → 404, visible TKT-V01 → 200.
- `_position` for TKT-V02 → `{current:2, total:5}`, prev/next within visible subset only.
- Boot with `write: [ticket]` no-read role → exits 1: `roles.weird: grants write on "ticket" without a covering read grant…`. (AC8)
- Unstamped principal on /api/ → 500 (fail-loud invariant preserved).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — readGate gained `ReadQuery` (one seam for list + sidebar) instead of duplicating verdict switches per consumer; error mapping reuses `writeGateError`
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`just lint`, `just arch-lint`, `just coverage-check` (74.5% total, thresholds
pass), `go test ./internal/...` all green. Docs: GUIDE-acl-security read-path
rewrite (both gates, search-ordering contract, policy invariant, caching, menu
decision, perf caveat, _position deferral), `docs/acl-security.md` regenerated,
`docs/security.md` policy-mode row + write⊆read semantics bullet updated.

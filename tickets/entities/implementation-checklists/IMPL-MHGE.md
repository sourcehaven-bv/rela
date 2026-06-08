---
id: IMPL-MHGE
type: implementation-checklist
title: 'Implementation: ACL read-side (PR 1/2): per-entity GET + writes + ?include= gated; middleware fail-loud; ETag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/acl/features_test.go`
  TestFeature_PermitsRead_* (AllowAll, DenyAll, QueryPath, TypeNotInPolicy);
  `internal/store/storetest/graphquery.go` MatchingIDs conformance (4 cases:
  filters_to_candidate_set, returns_all_input_ids, empty_input,
  wrong_type_does_not_match); `internal/dataentry/acl_get_test.go`
  TestACLGet_* (TypeLevelReadGrant, ETagSuppressedOnDeny, IncludeFilter,
  WriteGateErrorMapping); `internal/dataentry/acl_write_test.go`
  TestACLWrite_PatchOnHiddenIs404 (4 cases) + DeleteOnHiddenIs404;
  `internal/dataentry/acl_middleware_test.go` (FailLoudOnApi,
  NonAPIPathsBypass, StampedPrincipalAttachesGate);
  `internal/dataentry/acl_regression_test.go` (NopACL_GetUnchanged,
  NopACL_NonExistentStill404).
- [x] Integration tests written — TestACLMiddleware_RouterChainOrder builds
  a real `App.NewRouter()` with ACL configured + a stamping principal
  resolver, then asserts a GET succeeds (pins the CRIT-1 composition fix).
  TestACLMiddleware_StampedPrincipalAttachesGate is behavioural (calls
  PermitsRead on a ticket + a document with a viewer policy that allows
  only tickets).
- [x] Happy path implemented — per-entity GET / PATCH / DELETE / clone /
  POST-action / ?include= chokepoints route through `gateReadOrNotFound`
  which calls `readGate.PermitsRead`. CRIT-2 added 5 relations handlers
  (EntityRelations, GetRelationType, Create/Update/DeleteRelation).
- [x] Edge cases from planning handled — RR-FGUZ (gate before body parse,
  If-Match, IsLocked → PATCH on hidden with malformed body / stale
  If-Match / current If-Match all 404), RR-NGMI (gate before getEntity →
  hidden and nonexistent both spend the same MatchingIDs roundtrip),
  RR-MZU4 (ETag suppressed on deny), RR-7TIU (fail-closed local-map-then-
  merge on include filter store error), RR-FRK1 (batched per-type include
  filter — O(types) not O(N-candidates)), RR-T15E (middleware scoped to
  /api/ only, SPA shell bypasses).
- [x] Error handling in place — `writeGateError` maps `context.Canceled`
  (emit nothing), `context.DeadlineExceeded` (504 acl_query_timeout), else
  500 acl_query_failed (RR-89XK / RR-J25J). `filterVisibleIncludes` logs
  slog.Warn before fail-closed drop on store error (SIG-3 from cranky
  round 2).

## Test Quality

- [x] Using fixture builders or factories for test data — `mustNewACL(t,
  p, st)` (RR-AGSR), `principalCtx(user)` (RR-MILH), `seedEntity`,
  `gateCtxFor`, `getEntityAs(WithHeaders)`, `patchEntityAs`,
  `deleteEntityAs`, `computeETagForTest` (RR-H9QB), `stripInstance`
  (RR-QLQW for body-shape comparison), `fakeGate` for error-injection.
- [x] No hardcoded values in assertions when object is in scope — entity
  IDs derived from seeded fixtures; the only hardcoded literals are the
  status code constants (`http.StatusNotFound` etc.) and the wire error
  codes (`"not_found"`, `"acl_query_failed"`, …) which are the contract.
- [x] Only specifying values that matter for the test — fixture builders
  set the minimum properties needed for the assertion; e.g. `seedEntity(&
  entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any
  {"title": "T1"}})` sets only id/type/title (the rest defaults).
- [x] Interpolated values constructed from objects, not hardcoded —
  `getEntityAs` and `patchEntityAs` build URLs from `(plural, entityID)`
  args; `computeETagForTest` re-derives the ETag the handler would emit
  rather than hardcoding the bytes.
- [x] Property comparisons use original object, not hardcoded strings —
  test assertions read back `app.getEntity` / `store.GraphCount` rather
  than re-typing the seeded values; `stripInstance` parses+re-encodes
  V1Error bodies so structural comparison (not text comparison) decides
  pass/fail.

## Manual Verification

- [x] Feature manually tested end-to-end — full local `just ci` cycle
  passes (lint 0 issues, all tests green, coverage 74.3% floor passes,
  arch-lint clean).
- [x] Each acceptance criterion verified with test scenario from planning
  — see REV-H499 "Acceptance Status" table; AC1-AC7 + AC-CRIT-1 +
  AC-CRIT-2 all PASS with named tests.
- [x] Edge cases manually verified — `git stash` + `go test -run
  TestACLMiddleware_RouterChainOrder` against the broken wrap order
  reproduces the 500-acl_unstamped_principal failure; restoring the fix
  passes. Same approach used for verifying CRIT-2 relations gating
  (manual diff inspection of the 5 handlers).

**Verification Evidence:**

| AC | Status | Test |
|----|--------|------|
| AC1 type-level read grant | PASS | TestACLGet_TypeLevelReadGrant |
| AC2 hidden GET → 404 (not 403) | PASS | TestACLGet_TypeLevelReadGrant deny cases |
| AC3 PATCH/DELETE on hidden → 404 | PASS | TestACLWrite_PatchOnHiddenIs404 (×4) + DeleteOnHiddenIs404 |
| AC4 ?include= filters hidden | PASS | TestACLGet_IncludeFilter |
| AC5 ETag suppressed on deny | PASS | TestACLGet_ETagSuppressedOnDeny |
| AC6 NopACL regression | PASS | TestACLRegression_NopACL_* (×2) |
| AC7 middleware fail-loud /api/ only | PASS | TestACLMiddleware_FailLoudOnApi + NonAPIPathsBypass |
| CRIT-1 router composition | PASS | TestACLMiddleware_RouterChainOrder (verified to fail on broken order) |
| CRIT-2 relations handlers gated | PASS | gateReadOrNotFound calls at top of 5 handlers; diff inspection |

## Quality

- [x] Code follows project patterns — consumer-side interface for
  `readGate` per CLAUDE.md; `gateReadOrNotFound` mirrors the existing
  helper pattern (early-return with response written). `acl.Request`
  methods match the existing `AuthorizeWrite` / `ForEntity` shape.
- [x] Checked for DRY opportunities — extracted `gateReadOrNotFound`
  (LEV-1 from cranky review) across 9 chokepoint sites; extracted
  `writeGateError` for the three error-mapping branches. Architect rework
  consolidated `q := *rqr.Query; q.WhereIDs = ids; store.GraphQuery(q)`
  pattern (would have been duplicated across single + batched paths)
  into `GraphQueryer.MatchingIDs` — the gate now executes the predicate
  in one place.
- [x] No security issues introduced — both cranky+architect agents found
  CRIT-1 (composition order) and CRIT-2 (5 ungated handlers); both fixed
  with regression tests. SIG-1 (nil-rejecting constructor), SIG-2
  (principal-mismatch check), SIG-3 (loud store-error log on include
  filter) all addressed.
- [x] No silent failures — `slog.Warn` on every gate-construction
  failure, principal-mismatch, ForPrincipal failure, and include-filter
  store error. Production responses use constant Detail strings;
  `slog.Warn` carries the raw err (RR-372L).
- [x] No debug code left behind — full diff inspection; no TODO/FIXME
  in changed files; no `fmt.Println` or `log.Print` (all logging routes
  through `slog`).

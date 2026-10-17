---
id: IMPL-S3AHW7
type: implementation-checklist
title: 'Implementation: Free-text search judges hidden fields one hit at a time'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestSearchFieldScope_AgreesWithFieldVerdicts`, `TestRoleUnion_Monotonic`, the load-time lowering rule tests in affordances and acl)
- [x] Integration tests written (test full flow, not just units) (`storetest.RunFieldScopeSearchTests` on memstore, bleve, fsstore, sqlite and postgres; list `?q=`, `/_search` and MCP search run `visibility.SearchScoped`)
- [x] Happy path implemented (`search.Visible.fieldVisible` reads a page's rows in one `ListEntities` and answers each conditional grant with one `MatchingFaces` per type per page; pgstore computes the grants in the search SQL)
- [x] Edge cases from planning handled (stale index entry dropped; face-level hits judged on the matched face; role union with block-less roles; edge-granted roles via `LocalRolePredicate`)
- [x] Error handling in place (errors surfaced, not swallowed) (store errors wrap `search.ErrScope` and fail the search)

## Test Quality

- [x] Using fixture builders or factories for test data (`newBudgetAppOn`, `mustNewACL`, storetest fixtures)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (`TestQueryBudget_ListFreeTextUnderFieldHiding` drives the list handler with `?q=` at 10 and 50 hits and reads the store counter)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestQueryBudget_ListFreeTextUnderFieldHiding`: zero field-check entity loads and the same `MatchingFaces` count at 10 and 50 hits. On origin/develop it pinned one load and one traversal per hit.
- `TestListBodies_QueryShapesKeepTheBound`: `search.Service.Search` reads hit headers 256 at a time, so a narrow or broad `?q=` reads no body at 10 or 50 rows.
- `go test -race ./...` (default tags), the CI sqlite suite and the postgres-tagged suites pass; the only failure is the known `TestChromeStyle_TargetsShippedClasses`.
- `just lint` (all tag sets), `just arch-lint`, `just comment-lint`, `just plimsoll`: clean.

## Quality

- [x] Code follows project patterns (check similar code) (collection-reads rule: headers and batches per page, pinned by a budget test)
- [x] Checked for DRY opportunities (dataentry's `searchVisibleHits` merged into `visibility.Searcher`; one gated search path)
- [x] No security issues introduced (the matched-field check stays in Go; a matcher may leave a field out, never add one)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

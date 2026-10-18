---
id: IMPL-54ONM1
type: implementation-checklist
title: 'Implementation: sqlite short-needle search folds case for ASCII only'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code. `BackendFoldsLikeGo` in
`storetest.RunFieldScopeSearchTests`; a v18 case in
`TestSearch_MigrationRebuildsIndex`.
- [x] Integration tests written (test full flow, not just units). The storetest
case runs through `SearchVisible` with a field scope on every backend.
- [x] Happy path implemented. sqlitestore writes `entities.search_text` in Go
with `search.IndexText` (shared with pgstore) and searches it with a
case-sensitive trigram index and `instr`.
- [x] Edge cases from planning handled. Short needles, long needles, rename,
restore all rewrite `search_text`; `sqlitestore.New` fills it for existing
rows after the v19 rung.
- [x] Error handling in place (errors surfaced, not swallowed). Backfill and
write errors return from `New` and the write paths.

## Test Quality

- [x] Using fixture builders or factories for test data. The storetest harness
seeds through the store under test.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end. The storetest case failed on
sqlitestore before the fix for both needles and passes after it.
- [x] Each acceptance criterion verified with test scenario from planning. A
hidden-field match is no longer admitted for "é" or "ԩab".
- [x] ~~Edge cases manually verified~~ (N/A: covered by the conformance case and the migration test; no UI surface)

**Verification Evidence:**
`go test -tags sqlite ./internal/store/sqlitestore/` passes, including
`RunFieldScopeSearchTests/BackendFoldsLikeGo` and the v18 migration case;
the same case passes on memstore, fsstore and pgstore.

## Quality

- [x] Code follows project patterns (check similar code). Same Go-lowered
`search_text` design as pgstore.
- [x] Checked for DRY opportunities. pgstore and sqlitestore share
`search.IndexText`.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

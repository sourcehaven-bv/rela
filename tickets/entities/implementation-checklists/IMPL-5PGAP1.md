---
id: IMPL-5PGAP1
type: implementation-checklist
title: 'Implementation: rename.go upsertEntity/upsertRelation retain pre-BUG-ZWTDH9 create-then-Update-on-ErrConflict fallback (lost-update/clobber)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestRename_ConflictDoesNotOverwrite: entity + relation conflict subtests)
- [x] ~~Integration tests written~~ (N/A: the fix is a single package-local write helper; a conflictOnCreateStore unit test exercises the full Rename flow. Live multi-writer postgres race needs no new integration harness — the invariant is 'never Update on Create conflict', proven deterministically)
- [x] Happy path implemented (strict CreateEntity/CreateRelation; existing rewrite/delete flow unchanged — TestRename_RewritesRelations et al. still green)
- [x] Edge cases from planning handled (self-referential edges already handled by writeRenamedRelations skip; conflict now surfaces ErrEntityAlreadyExists for both entity and relation)
- [x] Error handling in place (store.ErrConflict mapped to ErrEntityAlreadyExists; non-conflict errors surface as-is)

## Test Quality

- [x] Using fixture builders or factories for test data (seedEntity/seedRelation helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test (conflictEntity/conflictRelation flags isolate each path)
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (go test -race ./internal/rename/ green; entitymanager suite green)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (confirmed the new test FAILS against the pre-fix upsert code — stashed rename.go, ran test → both subtests fail with 'store: not found' instead of ErrEntityAlreadyExists — then restored the fix and re-ran green. Proves it is a real regression guard)

**Verification Evidence:**
- `go test -race ./internal/rename/` → ok (1.4s)
- New test against OLD code → FAIL (entity + relation subtests), against NEW code → PASS
- `go vet` clean · `golangci-lint run ./internal/rename/` → 0 issues · `just arch-lint` → OK
- rename coverage 88.1%
- Diff: rename.go −31/+20 (upsert helpers deleted, strict createRelation added), rename_test.go +81

## Quality

- [x] Code follows project patterns (mirrors create-no-overwrite-test / BUG-R2PV8G conflictOnCreateStore wrapper)
- [x] Checked for DRY opportunities (two upsert helpers collapsed; entity create inlined at the single call site, relation create kept as one helper reused by both loops)
- [x] No security issues introduced (removes a lost-update/clobber footgun; no new surface)
- [x] No silent failures (the removed fallback WAS the silent failure — conflicts now returned, not swallowed)
- [x] No debug code left behind

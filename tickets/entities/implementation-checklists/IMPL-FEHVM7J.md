---
id: IMPL-FEHVM7J
type: implementation-checklist
title: 'Implementation: rela test v1 (1/4): untagged in-memory appbuild recipe with MemFS write / disk config split'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (SafeFS non-disk write tests, inMemoryFS/migrationLockDir table test, RejectsBadRoot)
- [x] Integration tests written (test full flow, not just units) (memrecipe_test.go builds full worlds over a disk project)
- [x] Happy path implemented (NewInMemory builds memstore + LinearSearch + MemFS + disk config)
- [x] Edge cases from planning handled (comments, piles, leases, state KV, migration record, migration lock, empty/non-project/file root)
- [x] Error handling in place (errors surfaced, not swallowed) (bad root returns error; prepare/assemble errors returned)

## Test Quality

- [x] Using fixture builders or factories for test data (writeMemWorldProject, memWorldAs, per-case newWorld)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (tree-hash test writes through every runtime-state writer and compares the project tree)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (CI dependency gate run locally; forbid fails on a real match)

**Verification Evidence:**

- `go test -run 'TestNewInMemory|TestInMemoryFS' ./internal/appbuild`: ok. The tree hash (path, mode, mtime, sha256) is unchanged after create, patch, comment, pile, lease, state KV and migration-record writes on a project with comments, piles and a pending data migration.
- goleak over 100 build+Close cycles: clean except the neoq cron runner (at most one per world; TKT-L98KKVX).
- Automation, validation (warning on /properties/title), ACL (acl.ErrForbidden for a viewer) and in-memory audit verified per case.
- `go test ./internal/storage ./internal/comments/... ./internal/datamigration/...`: ok.
- CI "Assert dependency isolation" and "conflicting backend tags" steps run locally: exit 0; postgres build of internal/appbuild links memstore and no bleve.

## Quality

- [x] Code follows project patterns (check similar code) (recipe shape matches the fs/sqlite recipes over prepare/assemble)
- [x] Checked for DRY opportunities (inMemoryFS centralizes six MemFS type checks)
- [x] No security issues introduced (rela-security-reviewer run; see review responses)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

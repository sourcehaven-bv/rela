---
id: IMPL-MQ0ION
type: implementation-checklist
title: 'Implementation: fs-to-sqlite migration command'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit tests: `internal/fsimport` (in-memory backend over real fsstore fixtures in
a temp dir), `storage.ReadOnlyFS`, `fsstore.Config.IgnoreIndexCache`,
`app.FSFactory.ReadOnly`, `filecomments.ThreadKeys`. Integration (sqlite tag):
`TestDBImportFS_SQLite` runs the command backend and opens the result with
`appbuild.At`; `TestDBImportFS_SQLiteTablesAreAccounted` fails on any new sqlite
table the import has not decided about.

Deviations from the plan:

- One untagged package `internal/fsimport` instead of `internal/backendcopy`.
The sqlite specifics (open, checkpoint, derived-index reconcile, WAL check) come
in through `fsimport.Backend`, so the package is tested without the sqlite tag.
- The audit record is written only on success. A failed run leaves no target
to hold it, and the source is never written.
- No validation report. Rows are copied as they are (AC8 "copied"); the
report tells the operator to run `rela-sqlite analyze` on the target rather than
wiring a second validator into the import.
- The `rela.db.lock` sidecar stays in the target; the sqlite build leaves it
after every close, so removing it gains nothing.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-sqlite db import-fs` on a copy of this repo's `tickets/` project.

- First run refused with 67 errors: relation files in `tickets/relations/`
whose frontmatter says `type:` instead of `relation:` (fsstore reads them with
an empty type today). Each was reported once, by path, with the missing key.
Nothing was written and no staging directory was left.
- After fixing those in the copy: exit 0 in about 5 s. 6086 entities and 7491
relations, equal to the file counts. Source sha sums identical before and after.
Target `.rela/` is 0700, has `rela.db` and the audit dir, no WAL/SHM.
`.gitignore` has `.rela/`. Audit record `op: fs-import`, principal `jeroen/cli`.
`rela-sqlite show TKT-YNKKRQ` and `list` work on the target; `analyze
cardinality` passes; `analyze validations` output is identical to the fs build's
on the source.

AC coverage: AC1 `TestRun_CopiesEverything`, `TestDBImportFS_SQLite` (verify
step re-reads the renamed target); AC2 source snapshot equality in the run tests
and the sha sums above; AC3 `TestRun_RefusesEncryptedContent`; AC4
`TestRun_RefusesBadPaths`; AC5 `TestRun_CollectsEveryRowError`,
`TestRun_FailsWhenTheSourceChanges`, `TestRun_BackendFailureLeavesNothing`; AC6
`TestRun_CopiesEverything`, `TestRun_LegacyMigrationMarker`; AC7 audit and
attribution assertions in `TestRun_CopiesEverything`; AC8
`TestRun_CollectsEveryRowError`, `TestRun_CaseCollidingIDs` (skips on a
case-insensitive filesystem, runs in Linux CI); AC9
`TestRun_ListsFilesTheStoreDoesNotRead`; AC10 mode and `.gitignore` assertions
in `TestRun_CopiesEverything`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

golangci-lint clean (default and sqlite tags, except the pre-existing unused
`noopSQLiteCloser` in an untouched file), `just arch-lint` clean, `just
comment-lint` clean, `just plimsoll` clean, race tests pass.

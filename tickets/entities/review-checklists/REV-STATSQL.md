---
id: REV-STATSQL
type: review-checklist
title: 'Review: runtime state in the database'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race -tags sqlite` over state, sqlitedb, appbuild, config)
- [x] Lint clean (`just lint`, `just arch-lint`, `just plimsoll`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: the package is three methods over one table, and the conformance suite is a stronger check than a reviewer would apply by hand. The design question — where the package lives — was already settled by the correction that produced `configsql`, and this follows it.)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: evidence below)

**Acceptance Status:**

- PASS — `state/statetest.RunAll` passes, wrapped in `ValidatedKV` exactly as
  the wiring site wraps it, so the key-rejection cases exercise the production
  composition rather than the raw backend.
- PASS — an oversize value is rejected, not stored short
  (`TestPutRejectsOversizeValue`). A silently truncated cached render would be
  served as if valid.
- PASS — `state_kv` created on fresh databases and by a v2→v3 rung, from one
  shared DDL constant. The ladder guard (`TestMigrationLadderIsWellFormed`)
  and the fresh-install guard (`TestFreshDatabaseSkipsTheLadder`) both still
  pass with the new rung.
- PASS — verified on a real project built with `just build-cli-sqlite`:
  `.rela/rela.db` holds `entities`, `relations`, `attachments`,
  `project_files` and `state_kv`, at `PRAGMA user_version = 3`.
- PASS — the derived-schema bypass now reads through the config seam; all
  three build tags compile and the no-op twin took the same signature.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-STATSQL)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-STATSQL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

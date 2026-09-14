---
id: REV-CFGSQL
type: review-checklist
title: 'Review: SQLite-backed config source'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race -tags sqlite` over store, appbuild, config, cli)
- [x] Lint clean (`just lint`, `just arch-lint`, `just plimsoll`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: reviewed inline against the two constraints that shaped it — see Acceptance. The design was revised twice during implementation as each constraint surfaced.)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: evidence below)

**Acceptance Status:**

- PASS — `config.Loader` conformance is DECLARED (`var _ config.Loader =
  (*Loader)(nil)`), so the compiler checks it. It could be declared only after
  the package moved out of `sqlitestore`; while it lived there, arch-lint
  forbade the import and conformance had to be structural.
- PASS — **the architecture was corrected mid-review**, and this is the
  finding worth recording. Config first went into `sqlitestore`, where it
  needed a duplicated `ConfigReader` interface, a test to keep the two in
  sync, and a build-tagged `layerStoreConfig` indirection — all of it
  scaffolding to work around arch-lint saying the code was in the wrong
  package. Extracting `internal/sqlitedb` (owns the file) and
  `internal/config/configsql` (reads config) deleted all three. A related bug
  went with it: `ProjectFiles()` had been on `Conn` but not `Store`, so the
  type assertion silently failed and the layer was never installed.
- PASS — absent row is `fs.ErrNotExist`-compatible
  (`TestProjectFiles_MissingIsNotExist`); a layered loader falls through on
  exactly that error and nothing else.
- PASS — `List` sorted, scoped, non-recursive, and literal:
  `TestProjectFiles_List` covers the first three (including that
  `scriptsnotadir.yaml` must not match `scripts`), and
  `TestProjectFiles_ListTreatsDirAsLiteral` covers the fourth — a `LIKE`/`GLOB`
  implementation would let `a_b` match `axb`.
- PASS — absent directory lists empty
  (`TestProjectFiles_ListAbsentDirIsEmpty`), matching the filesystem loader and
  the asymmetry `datamigration.LoadDir` depends on.
- PASS — both backends accept and reject the same names
  (`TestProjectFiles_RejectsUnsafeNames`, the same ten vectors as the
  filesystem loader's).
- PASS — disk wins per file and `List` unions both layers
  (`TestLayered_DiskWinsOverBakedConfig`, end to end over a real directory).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-CFGSQL)
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CFGSQL

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

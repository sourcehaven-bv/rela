---
id: PLAN-I8RHS9
type: planning-checklist
title: 'Planning: rela db dump / rela db load: export and import a project''s config (and data) to and from rela.db'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation: the work landed on feat/self-contained-sqlite
before this checklist was filled in. The config half of `rela db load` / `rela
db dump` shipped in 39b8e316 (committed under TKT-WFB1YH); the `--data` half in
d988168c. Each item states what was done.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: sqlite-build `rela db load [--from dir] [--data] [--force]` and `rela db
dump <dir> [--data]`. Config: `CollectProjectConfig`, `LoadProjectConfig`
(replaces the stored set through `configsql.Loader.Replace`) and
`DumpProjectConfig`. Data: `ImportMarkdownData` (one `store.Tx`, attributed to
the `fs-import` tool, one `fs-import` audit record, refuses a store that already
holds entities unless `--force`) and `ExportMarkdownData` (writes through a
plain fsstore). Non-sqlite builds report the commands as unavailable.

Out: the desktop File menu export/import (TKT-FGIWPE). Deviation from the
description: the source directory is the `--from` flag, not a positional
argument. Config `load` writes no audit record; only the `--data` import is
audited. `dump` only reads the database.

**Acceptance Criteria:**

1. Dump then load round-trips byte-identically. Test:
`TestSQLite_DumpRoundTripsAndRefusesOverwrite` (dumped files equal the loaded
bytes), `TestLoader_RoundTrip` (`internal/config/configsql`).
2. A markdown project converts into a self-contained rela.db and back. Test:
`TestImportMarkdownData_CopiesEverything`, `TestExportMarkdownData_RoundTrip`
(re-imported graph equals the original), `TestSQLite_BootsFromBakedConfig`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: follows the existing raw-store exceptions in CLAUDE.md)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: uses rela's own store and markdown code)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal command)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:**

- Perf seeding (`rela dev seed`) set the terms for a raw-store write:
operator shell, attribution, one audit record, refuse a non-empty store. The
import follows them.
- `configsql.Loader.Put` / `Paths` existed unused; `Replace` was added so a
load drops files removed from disk.
- The export reuses fsstore, so its output is what the fs build reads.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** the import reads the source with an fsstore and copies
entities, relations and attachments into the SQLite store inside one `Tx`, so a
failure rolls back and the import can be re-run. Rows are attributed with
`store.WithAttribution`.

Alternative rejected: routing the import through entitymanager. Automations
would rewrite data that already passed validation, as with perf seeding.
Alternative rejected: batched transactions. SQLite admits one process, so
nothing waits on a single long transaction.

**Files to modify:** `internal/appbuild/projectfiles_sqlite.go`,
`projectdata_sqlite.go`; `internal/cli/db.go`, `db_sqlite.go`, `db_postgres.go`,
`db_nonpostgres.go`; `internal/audit/audit.go` (`OpFSImport`);
`internal/config/configsql/configsql.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Files collected for `load`: an allowlist of top-level files and areas;
hidden files skipped; symlinks refused (`TestSQLite_CollectProjectConfig`,
`TestSQLite_CollectRefusesSymlinks`). `.rela/secrets.yaml` is never stored.
- Names read back for `dump`: re-checked, because a handed-over database may
hold a path that escapes the target (`TestSQLite_DumpRefusesEscapingNames`,
`TestSQLite_DumpRefusesSymlinks`).

**Security-Sensitive Operations:**

- Raw-store write without ACL or automations. The trust boundary is the
operator shell, as for `db migrate` and `history-purge`. The audit sink is
required (`TestImportMarkdownData_RequiresAudit`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above.

**Edge Cases:** a load after a file was removed from disk
(`TestSQLite_LoadReplacesTheSet`); `--force` beside stored rows
(`TestImportMarkdownData_ForceAddsBesideStoredRows`); legacy `metamodel.yaml`
stored as `schema.yaml`; attachments, relation properties and bodies.

**Negative Tests:** import into a non-empty store refused; `--force` still fails
on a stored id (`TestImportMarkdownData_NonEmptyDatabase`); dump refuses to
overwrite without the flag; export refuses existing `entities/`, `relations/` or
`attachments/` (`TestExportMarkdownData_RefusesExistingDataDirs`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Import skips validation, so invalid data in the source arrives as is.
Accepted: the source already passed or predates validation (CLAUDE.md).
- Timestamps are not carried over; the store stamps its own write time.
Documented in the godoc.
- A database from someone else can carry code. Mitigated: `dump` it and read
the config first (documented).

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `docs/sqlite-backend.md` and GUIDE-sqlite-backend
(commands and the markdown conversion), CLAUDE.md (fifth raw-store exception),
CLI help text.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: follows the sanctioned raw-store exception pattern; the code was covered by the branch-wide review recorded on TKT-FGIWPE)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none specific to this ticket.

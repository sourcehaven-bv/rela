---
id: REV-WFPQ1Z
type: review-checklist
title: 'Review: SQLite content versioning: entity + relation history, sweep, and purge behind the sqlite build tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` (default build) passes. The sqlite-tagged packages
(`./internal/store/...`, `./internal/sqlitedb/`, `./internal/appbuild/`,
`./internal/dataentry/`, `./internal/cli/`) pass under `-tags sqlite`.

pgstore was re-verified against a real PostgreSQL 17 with
`RELA_TEST_DATABASE_REQUIRED=1 go test -race -tags postgres
./internal/store/pgstore/...`: PASS with **0 skips**, which is the condition
the Postgres Backend CI job enforces (it greps for `--- SKIP` and fails).

One `just test` failure was mine and is fixed: a line in `DOCS-Z2EFSK.md`
began with `501.`, which `TestMdCorpusRoundTrip` correctly re-parsed as an
ordered list, so the file was not a render/parse fixed point.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-EISIAL, RR-LB664Z, RR-XUB1HS (critical, all
addressed); RR-HNQ4Q9, RR-FJHKLK (significant, both addressed); RR-C3OSDO
(minor, addressed); RR-BAPSQR (minor, wont-fix with reason).

The three critical findings were all real and all now carry a regression test
that was confirmed to FAIL against the original code before the fix:

- **RR-EISIAL** — purge deleted rows and wrote its sweep-suppression tombstone
  as two autocommit statements, so a crash between them left the erasure
  re-capturable. Both purge paths are now one transaction.
- **RR-LB664Z** — `CreateRelation` bumped the lineage counter before the
  INSERT, so every duplicate-triple `ErrConflict` (a routine outcome) burned an
  id permanently. The id is now read inline in the INSERT and consumed only
  after it succeeds.
- **RR-XUB1HS** — the sweep re-selected already-captured rows, so a backlog
  larger than `Batch` never drained: measured at 9 of 12 entities never
  receiving a version. Fixed by ordering on the capture time rather than
  filtering, because the obvious filter would have permanently excluded an edit
  landing in the same clock tick as its predecessor's capture.

RR-BAPSQR is the one finding I did not act on, and the reason is recorded on
the entity: pgstore pins the identical behaviour in its own tests, so changing
it here alone would break them and split the two backends under a conformance
suite whose purpose is that they agree.

**A background security review separately flagged a suspected "control
regression" in `purge.go`. It is a FALSE POSITIVE**, and it is worth writing
down so the next reviewer does not re-derive it. Reading two implementations
side by side is a poor way to establish a negative, so each control property is
now an executable claim in
`internal/store/sqlitestore/purge_guardrails_test.go`, and each was confirmed
to FAIL with its guard removed:

| Property | Verified |
|----------|----------|
| DryRun never deletes, and still populates `RenameInTargets` / `LiveRowExists` / `Targets` so the caller can render the reason | `TestPurgeDryRunNeverDeletes` |
| A rename row refuses even WITH `--force-live` | `TestPurgeRefusesARenameRowEvenWithForceLive` |
| A live row refuses without `--force-live` | `TestPurgeRefusesALiveRowWithoutForceLive` |
| `--force-live` on one face leaves a sibling face's history intact | `TestPurgeForceLiveIsScopedToOneFace` |
| An empty selector ERRORS rather than defaulting to a scope | `TestPurgeRefusesAnEmptySelector` |
| `--all` walks the fenced lineage, so an id reused after a rename keeps its own history | `TestPurgeAllUsesTheFencedLineage` |
| A multi-lifetime relation key refuses without a selector, deleting nothing | `TestPurgeRelationMultiLifetimeRefusalDeletesNothing` |

The guard ORDER also matches pgstore statement for statement: resolve targets,
set the flags, `DryRun` return, rename refusal, live-without-force refusal, and
only then the delete. The one genuine divergence is in the safe direction —
sqlite wraps the delete and its tombstone in a transaction where pgstore leaves
them as two autocommit statements (RR-EISIAL). The store performs no ACL check
on either backend by design; authorization for `history-purge` is the operator
shell, as it is for `db migrate`.

**Unrelated changes in the diff, each deliberate:**

- `internal/store/pgstore/relation_version.go` gains an exported
  `RelationRecordID`. Required by AC-2: without it the shared suite cannot
  address a relation's CURRENT lifetime on pgstore, and the multi-lifetime
  purge case skipped — which the Postgres CI job treats as a failure.
- `internal/store/pgstore/conformance_test.go` declares
  `Capabilities{Versioning}`. This is the point of extracting the suite.
- `internal/appbuild/` build tags are restructured (`capability.go` loses its
  postgres tag, `versionsweep_shared.go` and `statekv_nodb.go` appear). Forced:
  `versionServiceFor` and `stateKVFor` shared one `!postgres` file, so
  versioning could not be enabled for sqlite without also dragging in the
  database-backed state KV this ticket puts out of scope.
- `.github/workflows/ci.yml` adds `./internal/sqlitedb/...` and
  `./internal/dataentry/...` to the sqlite job — otherwise the new migration
  and history-endpoint tests would never run in CI.
- `.go-arch-lint.yml` lets `sqlitestore` depend on `canonical`. The rule's own
  comment said versioning was a later stage that "should have to justify
  widening this list"; this is that stage, and the justification is recorded
  inline.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all nine PASS. Evidence table is in IMPL-M7RLC8;
the two worth restating here:

- **AC-2 PASS** is the load-bearing one: `storetest.RunVersionTests` (31 cases)
  runs green on sqlitestore AND on pgstore against a real database, with no
  skips. Extracting it immediately earned its keep by surfacing the pgstore
  lifetime-resolution defect that RR findings above describe.
- **AC-1 PASS, and it was NOT passing when this review started.** The seven
  capabilities were fully implemented and their conformance suite passed while
  the feature was unreachable from a running app: `versionServiceFor` sat
  behind `//go:build !postgres` and returned nil, so every history request on
  the sqlite build returned 501 and the SPA hid the History button. A
  compile-time interface assertion would not have caught it — the store
  satisfied the interfaces throughout. The tests now ask the resolver
  (`TestSQLiteStoreSatisfiesTheVersionCapabilities`) and the HTTP handler
  (`TestHistoryIsServedOnSQLite`), which are the things that were wrong.

**Known limitation, accepted:** sweep-captured create/update versions always
carry the `version-sweep` system principal, because sqlitestore's live rows
have no `last_edited_by_*` columns to copy from. Synchronous captures
(rename/delete) carry the real principal. This meets AC-9's requirement —
never a guessed or literal-"unknown" identity — and the editing principal stays
recoverable from the audit log. Closing it means adding attribution columns to
the live `entities`/`relations` tables, which is a schema change out of scope
here; it is documented at the fallback in `sweep.go`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Z2EFSK

Four surfaces asserted that SQLite has no version history, all now false:
`CLAUDE.md`, `docs/sqlite-backend.md`, `docs/cli-reference.md` (six commands
marked "PostgreSQL build only") and five CLI help messages that named a
"PostgreSQL-build feature" — the last of which would have actively misinformed
a SQLite user whose backend does support it.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

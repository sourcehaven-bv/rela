---
id: REV-2LZVE7
type: review-checklist
title: 'Review: Replace the shape-hash migration chain with a committed applied-list; enable data-only migrations'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just ci` passes end to end (exit 0), including the docs-freshness gate.
Additionally verified: `just arch-lint` (the four new backend packages and their
appbuild dependencies are declared), `just plimsoll`, and the build-tag
isolation invariants — the postgres build links no bleve, the default build no
pgx, and only the sqlite build links `modernc.org/sqlite`.

Coverage **80.1%**, up from 79.8% on develop; all package floors satisfied. Two
exclusions added, matching the existing precedent: `migstatetest` (shared
conformance kit, exercised via the four backends, like `commentstest`) and
`pgmigstate` (live-DB gated, reads 8% when the suite skips and 77% in the
postgres CI job — measured, not estimated, like `pgcomments`).

**Comment findings:** `just comment-report` reports no advisory findings in any
file this diff touches. No suppressions were added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Both reviewers ran in parallel per the workflow. The security reviewer scoped
itself to the surfaces this diff touches and listed what it skipped as untouched
(ACL, visibility, entitymanager, dataentry handlers, cmdexec, mail, jobs, lua).

**Review Responses:**

| ID | Severity | Finding | Status |
|---|---|---|---|
| RR-QQLKDE | critical | LegacyBridge dropped unconvertible names, yielding an empty applied list that replays the chain | addressed |
| RR-SEIKCR | critical | `migrate data` guessed the stored shape when un-baselined, replanning everything | addressed |
| RR-NFXSZ7 | significant | `[security]` `migrate baseline` wrote without the migration lock | addressed |
| RR-5TQA14 | significant | `Persist` blind-overwrote a record that moved since `Evaluate`; `Verdict` aliased the loaded slice | addressed |
| RR-9VVG5H | significant | Two `hasMigrations` probes disagreed on the un-baselined decision | addressed |
| RR-I2FPYC | minor | `[security]` LegacyBridge skipped `ValidateState` | addressed |
| RR-HP70BX | minor | Batch: fragile length constant, missing live hash, unused error type, inaccurate test claim | addressed |

The two criticals were the same mistake in two places: code that documented the
danger of guessing, then guessed. Both are now refusals. I reproduced RR-QQLKDE
with a throwaway test before fixing it, and verified the regression tests for
RR-SEIKCR, RR-NFXSZ7 and RR-5TQA14 genuinely fail when their fix is reverted.

**Self-review:** every changed file is in scope — `internal/datamigration` and
its four backends, the appbuild/CLI wiring, and the two DDL rungs. No TODOs,
FIXMEs, stray panics or debug prints in the diff.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **Data-only migration applies, is recorded, skipped on re-run** — PASS.
`TestMigrateData_DataOnlyMigrationRunsAndIsRecorded` plus
`TestParseFile_DataOnlyMigration`; verified end to end against a real project.
2. **`migstatetest.RunAll` passes for all four backends** — PASS, postgres
included, against a live database.
3. **Two postgres tenants migrate independently** — PASS,
`TestTenantIsolation` against a live database.
4. **A server never writes migration state** — PASS,
`TestGate_EvaluateDoesNotPersist`, plus `appbuild/datamigration.go` calling only
`Evaluate`.
5. **Non-empty `migrations/` with no record refuses** — PASS, and *strengthened*
by review: `TestMigrateStatus_RefusesWhenUnbaselined` **and**
`TestMigrateData_RefusesToGuessWhenUnbaselined`. Originally only `status`
refused while `data` guessed — RR-SEIKCR.
6. **sqlite keeps the record in `rela.db`** — PASS, conformance over a real
database file.
7. **`validateDeltasResolved` still refuses unmigrated face adoption** — PASS,
`TestParseFile_RefusesUnmigratedFaceAdoption` and
`TestResolvingSteps_CoversEveryMigrationDeltaKind` pass with no assertion
changes.
8. **A rollback finds a current legacy marker** — PASS,
`TestLegacyBridge_MirrorsWritesForRollback`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Z4OIOC

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Ten commits, each scoped to one concern and explaining the reasoning rather than
restating the diff. The review-fix commit names both defects and why each was
wrong, so the history carries the argument and not just the outcome.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A at review time:
`/pr` gates on the ticket already being `done`, so the PR post-dates this
checklist — see TKT-UFV01M)

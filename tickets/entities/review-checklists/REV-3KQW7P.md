---
id: REV-3KQW7P
type: review-checklist
title: 'Review: Composition-root tests fail under -tags postgres; CI compiles the tag but never runs or lints it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — full default suite green (83 packages); postgres-tagged `internal/appbuild/...` + `internal/cli/...` green against PostgreSQL 15, including `-race` (the exact new CI step); memorybackend build + tests green
- [x] Lint clean — `golangci-lint run ./...` **and** `--build-tags postgres ./...` both 0 issues (the tagged run was the point: it was never run before); `just arch-lint` OK; God-object lint OK
- [x] Coverage maintained — `just coverage-check` exit 0, package + total floors satisfied (77.7%)

## Code Review

- [x] Self-reviewed the diff for unrelated changes — the only non-test source edit is a lint comment in `versionsweep_postgres.go`; no production behaviour changes
- [x] ~~All critical review-responses addressed~~ (none raised)
- [x] ~~All significant review-responses addressed~~ (none raised)

**Two judgement calls worth recording:**

1. **G706 on `sweepConfigFromEnv`.** First fix added a `sanitizeForLog` helper.
   Removed it: `internal/dataentry` already establishes — pinned by
   `TestSlogTextHandlerEscapesNewlines` — that a constant message plus user data
   as a structured *attribute* is safe, because slog's handler quotes and
   escapes it. The sanitizer implied a protection that already existed, so a
   `nolint` citing that invariant is the honest fix.

2. **Skip-vs-fail.** Skipping without a database risks turning 13 real tests
   into 13 silent no-ops (RR-0EWZQW). Mitigated by reusing pgstore's
   `RELA_TEST_DATABASE_REQUIRED` rather than inventing a second variable, and by
   keeping the three tests that assert pre-store boot failures **ungated** so
   they still genuinely run with no database present.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented

**Verified, with fail-before checks rather than assumed:**

| Claim | Evidence |
|---|---|
| 13 previously-failing tests now pass | reproduced red on `develop` first, then green with a live DB |
| Corrupt-alias test is non-vacuous | mutated `buildStateAndAliases` to serve an empty table → test fails as intended |
| New CI step catches a real regression | nulled the state KV on the postgres wiring path (compiles, passes untagged) → step fails "State is nil" |
| Strictness gate fires | `RELA_TEST_DATABASE_REQUIRED=1` with no DSN → hard failure, not skip |
| Skip path is clean | no DSN → 3 tests still run, backend-reaching ones skip |
| No schema leakage | 0 `relawiring_%` schemas left after ~40 runs |
| Dependency isolation intact | `go list -deps`: pgx absent from default build, bleve absent from postgres build |

## Documentation (enhancements only)

- [x] ~~User-facing documentation updated~~ (N/A: CI/test-infrastructure fix, no user-visible surface)
- [x] ~~Docs-checklist created~~ (N/A: not an enhancement or docs ticket)

## Final Checks

- [x] Commit message explains the why (root cause: tag coverage scoped to the package that motivated the tag, not everything the tag changes)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `backendtest` is documented at package level, including why the skip policy is what it is

## Pull Request

- [x] PR created and CI monitored
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1362

**One scoping decision made during CI iteration.** The first version of the new
step ran `./internal/cli/...` whole and failed on
`TestRenderCmd_WritesRenderedFile`: that test shells out through
`internal/cmdexec`, which fails closed without a sandbox, and this job's runner
cannot create bubblewrap's loopback device (`RTM_NEWADDR: Operation not
permitted`) even after installing the package — unlike the `test` job.

The step now names the MCP wiring tests explicitly. The render test is
backend-independent and already covered by the `test` job, so gating the
postgres wiring on an unrelated sandbox capability bought nothing. Deliberately
NOT fixed by relaxing the sandbox or adding a skip inside the test — both would
weaken a real confinement guarantee to satisfy a job that does not need it.
Verified the narrowed filter still runs all three originally-failing CLI tests.

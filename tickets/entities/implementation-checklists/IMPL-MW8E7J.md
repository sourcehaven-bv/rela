---
id: IMPL-MW8E7J
type: implementation-checklist
title: 'Implementation: Database-backed comment stores: pgcomments and sqlitecomments over an injected pool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Integration is genuine on both backends: pgcomments runs against a real
PostgreSQL (per-test schema, created and dropped), sqlitecomments against real
`rela.db` files opened through `sqlitedb.Open`. Every edge case listed in
PLAN-6QD0LA § Edge Cases has a named test.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The conformance suite (`commentstest.RunAll`) is the main fixture factory: each
backend supplies a `func(*testing.T) comments.Store` and inherits every case, so
a new backend cannot quietly assert less than an existing one. Package-local
tests add small `add(...)` helpers rather than repeating comment literals.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Run against a real local PostgreSQL 16 with `RELA_TEST_DATABASE_REQUIRED=1`, so
a silent skip would have failed rather than passed.

| AC | Evidence |
|----|----------|
| 1 | `TestConformance` passes in both packages, `comments.Store` unchanged |
| 2 | conformance concurrency case green under `-race` on both |
| 3 | `TestCrossProcessVisibility` — a second, independent `pgxpool` against the same schema reads what the first wrote |
| 4 | `TestSchemaIsolation` — a comment in schema A is invisible in schema B |
| 5 | `pgstore.Open` no longer accepts a DSN; deleting the form means a missed call site does not compile |
| 6 | `TestCommentsSurviveReopen` — comments survive close + reopen of `rela.db`; `state.KV` wiring untouched |
| 7 | `TestBuildComments_{NilBackendUsesFilesystem,BackendOverrideIsUsed,DisabledYieldsNilService}` |
| 8 | `just arch-lint` → "OK - No warnings found" |

Full gate run:

- `go test ./internal/comments/... ./internal/appbuild/` — all green
- `go test -race -v ./internal/comments/...` with a live DSN — **zero skips**
- `go test -tags postgres ./internal/appbuild/` — green
- `go test -tags sqlite ./internal/appbuild/ ./internal/sqlitedb/` — green
- `just arch-lint` — OK, no warnings
- `golangci-lint` over all touched packages — 0 issues
- `just comment-lint` — no unresolvable doc links across 14964 comments
- plimsoll over all touched packages — exit 0
- coverage — package floor (50%) and total (65%) both PASS, total 79.5%

**Mutation-checked, not just observed green.** The override test is the one
guarding the ticket's actual defect, so it was verified by breaking the wiring
(`store := backend` → `var store comments.Store`) and confirming it fails, then
reverting. A test that cannot fail is not evidence.

**Two problems this step found that a green run would not have.**

1. *The pgcomments suite never ran in CI.* The Postgres job enumerates packages
explicitly (`./internal/store/pgstore/...`, `./internal/jobs/...`), and
pgcomments needs no build tag — it takes an injected handle, so it compiles in
every build and the untagged job silently skipped every meaningful test. The
parity gate would have existed and never fired. Added a
`./internal/comments/...` step with the same `--- SKIP` backstop the other two
DB-gated suites use, verified locally to run with zero skips.

2. *The coverage floor would have broken the default CI job.* pgcomments reads
3.4% when the suite skips, against a 50% package floor. Excluded on exactly the
terms pgstore already is, and the exclusion was verified NECESSARY (removing it
reproduces the FAIL) rather than added speculatively.

**Pre-existing local failure, not caused by this change:** `just plimsoll` and
`just coverage-check` both fail on `cmd/rela-desktop [build failed]` — an
unaccepted Xcode license breaking `runtime/cgo`. Confirmed pre-existing by
running `just coverage-check` on a stashed tree, where it fails identically.
Scoped runs of both tools over the touched packages pass, and CI runs on Linux.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: consumer-side `DBTX` interfaces declared per package
(CLAUDE.md), constructors reject nil required collaborators, injected handle
rather than a DSN (`pgstore.New(db DBTX)`), conformance suite for a new backend
implementation.

DRY: the shared behaviour lives in `commentstest`, not in copied assertions. The
two backends deliberately do NOT share an implementation — the SQL genuinely
differs (`$n` vs `?`, `JSONB` vs `TEXT`, `substring(... from ...)` vs `substr`),
and the timestamp format differs for a load-bearing reason, so a common base
would have to be parameterised past the point of being clearer than two files.

Security: the one new input surface is the entity id reaching SQL as a LIKE
pattern, escaped with an explicit `ESCAPE '\'` and pinned on both backends by
`TestRenameDoesNotMatchLikeWildcards`. Every statement is parameterised. No
credential handling is added — both backends receive a live handle and never see
a DSN.

No silent failures: zero rows affected on `Update`/`Delete` returns
`comments.ErrNotFound` rather than succeeding quietly, and every driver error is
wrapped with the target key and returned.

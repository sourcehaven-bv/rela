---
id: REV-RT9NXA
type: review-checklist
title: 'Review: Database-backed comment stores: pgcomments and sqlitecomments over an injected pool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run with a live PostgreSQL and `RELA_TEST_DATABASE_REQUIRED=1`, so a silent skip
would have failed rather than passed:

- `go test -race -v ./internal/comments/...` — green, **zero skips**
- `go test -tags postgres ./internal/appbuild/` — green
- `go test -tags sqlite ./internal/appbuild/ ./internal/sqlitedb/` — green
- `go test ./internal/store/pgstore/` — green (full suite, incl. the migration
ladder and `TestStatusFreshSchema` at target 15)
- `golangci-lint` over every touched package — 0 issues
- `just comment-lint` — no unresolvable doc links across 14964 comments
- `just arch-lint` — OK, no warnings
- plimsoll over every touched package — exit 0
- coverage — package floor (50%) and total (65%) both PASS, total 79.5%

**Comment findings.** `just comment-report` surfaced exactly one advisory
finding introduced by this diff: `nil-contract` on `buildComments`, which states
nil behaviour as prose. **Fixed, not suppressed** — the function has two
distinct nil contracts (a nil *return* meaning "feature disabled", and a nil
*backend argument* selecting filecomments), and both now use the standard `Nil:`
form. Re-run is clean. No suppressions were added anywhere in this diff.

**Dependency-isolation assertions** (the guarantee `NewPool` returning `DBTX`
exists to protect) re-verified by hand, all three passing: the default build
links no pgx, the postgres build links no bleve, and no build but sqlite links
`modernc.org/sqlite`.

**Pre-existing local failure, not from this change.** `just plimsoll` and `just
coverage-check` both fail on `cmd/rela-desktop [build failed]` — an unaccepted
Xcode license breaking `runtime/cgo`. Confirmed pre-existing by running `just
coverage-check` against a stashed tree, where it fails identically. Scoped runs
of both tools over the touched packages pass; CI runs on Linux, where this does
not arise.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers were run in parallel over `git diff develop...HEAD`:
cranky-code-reviewer (general correctness, concurrency, SQL, resource lifecycle)
and rela-security-reviewer (rela's own security invariants).

**Review Responses:** nine, all triaged. Both criticals and all three
significants are fixed and pinned by tests verified to fail without the fix; the
four minors are two fixed, one documented-not-fixed, two deferred with reasons.

| ID | Severity | Status | Finding |
|----|----------|--------|---------|
| RR-ZH02FY | critical | addressed | Rename aborted on a PK collision, stranding the whole thread |
| RR-YBCEOC | critical | addressed | Byte-vs-character offset lost every comment on a multi-byte id |
| RR-JKCF1G | significant | addressed | SQLite's case-insensitive LIKE reached an entity the caller never named |
| RR-GZXGAK | significant | addressed | pg timestamps came back in the server's local zone, diverging from sqlite |
| RR-Q8S0VY | significant | addressed | Pre-existing duplicate-id merge in filecomments and memcomments |
| RR-WXVDR8 | minor | addressed | Three comments asserted things that were not true |
| RR-XK5L66 | minor | wont-fix | MaxPerTarget is advisory once several processes share a database |
| RR-PAF29J | minor | deferred | No migration of existing .rela/comments/ YAML |
| RR-1T6I3M | minor | deferred | comments.Store has no single-comment read |

The two criticals were the same defect class the ticket exists to fix: silent,
permanent comment loss on the entity rename path, invisible because
entitymanager LOGS a failure from EntityRenamed rather than returning it. Both
were reproduced against a live database before being fixed.

RR-Q8S0VY is the one neither reviewer found. The rename-collision test written
for RR-ZH02FY failed on filecomments and memcomments too, for a different
reason: they merged with a blind append and produced a thread holding one
comment id twice. That predates this ticket. It is the argument for the
conformance suite in one line — a case added to fix one backend found the same
class of bug in two others nobody was looking at.

Two findings were REJECTED after checking rather than accepted on the reviewer's
word. `COLLATE BINARY` was the proposed fix for RR-JKCF1G; it does not affect
LIKE at all, which I confirmed by applying it and watching the fold persist, so
the fix is a byte-exact substr guard instead. And two conformance cases had to
be narrowed to the database backends after they failed on filecomments for sound
reasons — a case-insensitive filesystem cannot keep "TKT-1" and "tkt-1" apart,
and a file backend is right to refuse a non-ASCII id rather than build an unsafe
path.

### Verification done beyond the automated gates

- **Mutation-checked the load-bearing test.** `TestBuildComments_BackendOverrideIsUsed`
guards the ticket's actual defect, so it was verified by breaking the wiring
(`store := backend` → `var store comments.Store`) and confirming the test fails,
then reverting. A test that cannot fail is not evidence.
- **Verified the coverage exclusion is NECESSARY**, not speculative: removing
it reproduces a package-floor FAIL at 3.4%.
- **EXPLAIN-checked both new indexes** against 20k seeded rows.
`comments_target_prefix_idx` serves the faced-thread LIKE as a range scan
(`target_key ~>=~ 'TKT-3@' AND ~<~ 'TKT-3A'`), which is what makes `Rename` and
`DeleteAllFaces` cheap. `comments_thread_idx` is not chosen on a small thread —
the planner prefers the prefix index plus a ~40-row sort, which is correct — but
IS chosen unprompted on a fat thread (20k comments on one target), where it
supplies the ordering with no Sort step. The index earns its place exactly on
the threads that would otherwise hurt.
- **Checked the two backends agree where the conformance suite does not force
them to.** Both `Update` statements set exactly `body`, `resolved`,
`updated_at`, leaving `author`, `created_at` and `anchor` untouched — an edit
cannot rewrite who said something or what it was about.
- **Considered an id collision on rename-merge** (the destination already
holding a comment with the same id, which `PRIMARY KEY (target_key, id)` would
reject) and wrongly dismissed it as unreachable, on the grounds that comment ids
carry 80 bits of `crypto/rand` entropy. **The review overturned this** and it
became RR-ZH02FY, a critical: the entropy argument is about two ids colliding by
CHANCE, but a restored backup, a re-import or a filecomments migration puts an
existing id into a new thread deliberately. Odds were the wrong axis — the right
question was what happens WHEN it collides, and the answer was a stranded
thread. It is tested now.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1. Both pass `commentstest.RunAll`, interface unchanged | PASS | `TestConformance` in both packages; `comments.Store` untouched in the diff |
| 2. Concurrent adds all survive, enforced by the database | PASS | conformance concurrency case, green under `-race` on both |
| 3. Cross-process visibility on postgres | PASS | `TestCrossProcessVisibility` — a second, independent `pgxpool` reads what the first wrote |
| 4. Rows scoped to the tenant's schema | PASS | `TestSchemaIsolation` — a comment in schema A is invisible in schema B |
| 5. `Open` takes an injected handle; one pool feeds store + search + comments | PASS | the DSN form is deleted, so a missed call site does not compile; postgres recipe builds one pool and injects it three ways |
| 6. sqlite comments in `rela.db`; `state.KV` stays on the filesystem | PASS | `TestCommentsSurviveReopen`; `state.KV` wiring untouched in the diff |
| 7. default/memory still filecomments; no block still creates nothing | PASS | `TestBuildComments_{NilBackendUsesFilesystem,BackendOverrideIsUsed,DisabledYieldsNilService}` |
| 8. `just arch-lint` passes; neither package depends on `internal/store` | PASS | "OK - No warnings found" |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RTSUPP

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

**One gap this review found that no automated gate would have.** The pgcomments
suite was never going to run in CI. The Postgres job enumerates packages
explicitly (`./internal/store/pgstore/...`, `./internal/jobs/...`), and
pgcomments needs no build tag — it takes an injected handle, so it compiles in
every build and the untagged default job skipped every meaningful test. The
parity gate would have existed and never fired, which is the same class of
problem RR-0EWZQW records. Added a `./internal/comments/...` step to the
Postgres job with the same `--- SKIP` backstop the other two DB-gated suites
use, and verified locally that it runs with zero skips and that the env guard
hard-fails when the DSN is absent.

## Post-review verification

Re-run after every fix, with a live PostgreSQL and
`RELA_TEST_DATABASE_REQUIRED=1`:

- `go test -race ./internal/comments/... ./internal/appbuild/ ./internal/sqlitedb/` — all green
- `go test -tags postgres ./internal/appbuild/` and `-tags sqlite` — green
- `golangci-lint` over every touched package — 0 issues
- `just arch-lint` — OK, no warnings
- `just comment-lint` — clean across 14978 comments

Two lint findings arrived with the fixes and were fixed, not suppressed: gosec
G202 on the SQL concatenation (`selectFaces` became a `const`, which also makes
it plainly non-varying) and a `honour`/`honor` misspelling.

Every new test was mutation-checked — reverted to the pre-fix code and confirmed
to fail, then restored. A test that cannot fail is not evidence.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

---
id: REV-35FN6S
type: review-checklist
title: 'Review: ensurePoolFloor re-encodes the whole DSN query'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` reports the same 5 pre-existing issues before and after the diff,
none of them in a changed file. `just arch-lint` OK, `just comment-lint` clean.
Default `go test ./...` passes. With `-tags postgres` against a real database:
`internal/jobs` ok (125s), `internal/store/pgstore` ok (58s),
`internal/dataentry`, `internal/tenant`, `internal/docscapture` and
`internal/appbuild` all ok.

The pre-commit hook rejected the first commit attempt. That was
`TestAnalyzeProperties_StopsScanningAtCap` exceeding the 10m `just test`
timeout, not this diff: the same single test takes 469s on a pristine
`add75648` worktree with none of these changes, and 5 of the 6 changed files
are behind `//go:build postgres` so they are not in the default build at all.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

Self-review caught one defect in my own working copy before it was committed. I
had rewritten the existing-value branch's condition from
`convErr == nil && n >= floor` to `convErr != nil || n >= floor`, which would
have left an unparseable `pool_max_conns` untouched instead of raising it to the
floor. That is a behaviour change, and this diff is meant to change only
encoding. Restored the original semantics and left a comment saying an
unparseable value is raised deliberately.

Checked the diff for scope creep: `ensurePoolFloor` still decides exactly what
it decided before (absent, too small, unparseable, or already large enough), and
`replaceRawParam` only substitutes one pair. The four test-helper changes are
mechanical conversions to the key/value DSN form.

Audited the rest of the tree for the same hazard. No other DSN is built by
re-encoding a query: the production helpers in `internal/tenant/dsn_postgres.go`,
`internal/docscapture/scratch_postgres.go` and
`internal/appbuild/backendtest/backendtest_postgres.go` already re-serialize as
key/value. The remaining `url.Values.Encode` calls build HTTP queries, not DSNs.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | `options=-c search_path=x` survives pool sizing | PASS | `TestEnsurePoolFloor_PreservesOptionsEncoding` compares `pgconn.ParseConfig` output before and after |
| 2 | the same holds on the replace branch, not just the append branch | PASS | `TestEnsurePoolFloor_RaisesWithoutReEncoding` asserts `MaxConns=14` and an intact `options` via `pgxpool.ParseConfig` |
| 3 | the new tests fail on the old implementation | PASS | reproduced the exact CI error locally against the unfixed code: `unrecognized configuration parameter "+search_path" (SQLSTATE 42704)` |
| 4 | the 12 tests failing on PR #1589 pass | PASS | `internal/jobs` and `internal/store/pgstore` green with `-tags postgres` |
| 5 | existing `ensurePoolFloor` behaviour is unchanged | PASS | the pre-existing table test and the two preservation tests still pass untouched |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — internal bug fix, no user-facing surface.

The documentation that matters is the doc comment on `ensurePoolFloor`, which
records why the whole query must not be re-encoded. Without it the next person
simplifies `replaceRawParam` back into `u.RawQuery = q.Encode()`, because that
is the obvious way to write it and the reason it is wrong is invisible at the
call site.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: the CI failure on PR #1589 looked like a dependency
incompatibility, and stopping at "pin pgx" would have been enough to make CI
green. It would also have left a live defect in place. The tests only failed
because pgx stopped compensating for a corruption rela was already producing,
so the bump was the messenger. The lesson is to ask what a dependency's
behaviour change stopped hiding, not just what it broke.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

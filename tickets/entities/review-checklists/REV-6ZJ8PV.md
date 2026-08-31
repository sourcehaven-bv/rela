---
id: REV-6ZJ8PV
type: review-checklist
title: 'Review: testIdempotencyFreed races the queue completion bookkeeping'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just lint` 0 issues; `just arch-lint` clean; `just comment-lint` no
unresolvable doc links; `just plimsoll` clean. `go test ./internal/jobs/...
-race` green (69s), and the previously-failing case passes 20/20 under
`GOMAXPROCS=1`.

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

The diff is one call site plus its comment. Self-review checked the two things
that could be wrong with a polling fix:

1. **Can it still fail?** Yes — blocking the handler so the job never completes
still fails, after the full `settleTimeout`. A polling fix is only worth having
if it cannot pass when the property does not hold; otherwise it silences the
flake by deleting the assertion.
2. **Is the polled operation safe to repeat?** Yes. `Enqueue` with an
idempotency key is idempotent by construction — that is the feature under test.
A rejected attempt has no side effect, so retrying cannot enqueue duplicate
work.

Also checked the sibling `testIdempotencyCollapses` for the same defect: it
holds the handler open and asserts the duplicate IS rejected while the first is
pending. That direction asserts the state that exists DURING the window rather
than after it, so it has no race.

No unrelated changes.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | the previously-failing case passes under load | PASS | 20/20 under `GOMAXPROCS=1` |
| 2 | the test still detects a genuinely never-freed key | PASS | handler blocked → FAIL after settleTimeout |
| 3 | no production behaviour changed | PASS | diff is test-only |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — this is a bug fix in a test.

The documentation that matters is the comment at the call site, which records
why the polling is there and which two alternatives were rejected. Without it
the next person removes the "unnecessary" retry: the shape looks over-cautious
precisely because the race it prevents is invisible at full parallelism.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: my first conclusion was that this was a flake, and I re-ran CI
on the strength of it. The rerun failed identically, which disproved it.

The hypothesis never explained the evidence — "flaky" predicts randomness, and
this was two-for-two on one branch and zero elsewhere. What broke it open was
reaching for LOAD rather than ordering: `GOMAXPROCS=1` instead of another
`-shuffle` seed made it deterministic in a single run.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->

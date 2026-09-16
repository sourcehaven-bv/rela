---
id: REV-IKMAKL
type: review-checklist
title: 'Review: memory locker entry-count race'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test -race ./internal/lock/` passes. The specific test went from 8 failures
in 20 separate runs to 0 in 30 under `-race`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

The first question a polling fix has to answer is whether it can still fail, so
I mutation-tested rather than re-ran: deleting `l.drop(key, e)` from the
abandoned path in `memlock.go` still fails the test after 5 seconds. A polling
fix that cannot fail is just a deleted assertion.

I also checked whether the production code was the thing to change. It is not.
`Acquire` unlocks before dropping the refcount so the next waiter is unblocked
as early as possible; reversing that to make the test simpler would hold the
mutex across a map write on the contended path. The test's assumption about
ordering was the defect.

Checked the two sibling assertions in the same file. `TestMemoryLocker_NoEntryLeak`
and `TestMemoryLocker_ConcurrentDistinctKeys` read the count after fully
synchronous releases, where it is settled, so they are correct as written and
were left alone.

Diff is test-only. `memlock.go` is byte-identical.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | the test passes reliably | PASS | 30/30 under `-race` |
| 2 | it still detects a leaked refcount | PASS | `l.drop` removed -> FAIL after the poll deadline |
| 3 | no production behaviour changed | PASS | `git diff` on `memlock.go` is empty |
| 4 | the flake predates this branch | PASS | 8/20 failures on pristine `origin/develop` with pgx 5.10.0 |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — a test-only fix.

The comment above the poll records that the cleanup unlocks before it drops,
and gives the observed failure rate. The rate is the part worth writing down:
without it the polling looks like defensive habit rather than a response to a
measured 40 percent failure.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: this surfaced as a failure on the pgx bump PR, which made a
dependency the obvious suspect. It reproduces on pristine develop with no bump,
so the bump was coincidence. Checking that before investigating cost one
command and would otherwise have sent me through `internal/lock` looking for a
pgx interaction that does not exist.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

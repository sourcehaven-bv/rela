---
id: REV-4Z51JQ7
type: review-checklist
title: 'Review: Threads 1: read one thread and the top-level stream'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Tests: `just coverage-check` runs the full Go suite; the postgres-tagged pgstore
and pgchannels suites pass against a local database. Lint: golangci-lint clean
on every changed package (one run at a time); `just arch-lint` clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-AX3C8UX, RR-H3HMCF4, RR-H5EQ73I, RR-JH725LL,
RR-X5HFJ1Z, RR-XVG7URE, RR-UUOE7BZ, RR-RT6IR9L, RR-XTJDE9Q, RR-8GBF1L2. The one
significant finding (thread query skipped its index) is fixed with EXPLAIN tests
on both SQL backends; one minor is deferred and one nit is wont-fix, each with a
reason. The security reviewer found nothing above nit.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: channelstest ThreadPages on file, sqlite and postgres.
2. PASS: ThreadPages ("hidden replies never thin a full page"), filters in the store query.
3. PASS: ThreadPages, ThreadRemoval, LastReplyAtNeverBack.
4. PASS: realtime TestRead_Threads compares the error of a thread read and a plain read of a hidden anchor.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: user docs are slice 5, TKT-771TAL2)
- [x] ~~User-facing documentation updated~~ (N/A: slice 5, TKT-771TAL2; godoc and tool descriptions updated here)
- [x] ~~Docs-checklist marked as done~~ (N/A: slice 5)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

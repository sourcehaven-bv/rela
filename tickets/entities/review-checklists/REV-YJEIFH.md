---
id: REV-YJEIFH
type: review-checklist
title: 'Review: Postgres job queue cannot initialize against a schema-pinned DSN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: fix landed inside PR #1444's own
review cycle; the change is reviewed as part of that PR)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Self-review found and corrected a factual error in the original triage: the
ticket claimed the failure reproduced on unmodified `develop`. It cannot —
`git ls-tree origin/develop internal/jobs` is empty, so `develop` has no job
queue. The comparison run must have used a branch-built binary. The ticket now
records this as introduced by TKT-YOED3R rather than pre-existing, because a
wrong "pre-existing" label is what would have let it ship.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

Both acceptance criteria PASS — evidence in IMPL-YJEIFH. 265/265 e2e with a
database configured; postgres job-queue conformance green.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One thing a reader should know: the `replace` directive in `go.mod` is a
**temporary vendored fork**, and its comment says so along with the branch it
points at. `TestPostgresQueue_SchemaPinnedDSN` is what fails if it is ever
dropped before the fix is upstream.

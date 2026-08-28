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

## Adjacent CI failures found while verifying

Two failures surfaced that are NOT this bug and are recorded here so they are
not mistaken for it later:

1. **`TestFSLock_ConcurrentStaleBreakSingleWinner`** (`internal/datamigration`)
   failed about one `just coverage-check` run in three. Pre-existing on
   `develop` and untouched by this PR. `breakIfStale` re-verified staleness
   under the break mutex with `isStale`, which reports a MISSING file as
   stale, then removed unconditionally — so a breaker reading the path during
   the gap between another breaker removing the stale file and the winner
   creating its own could delete the winner's LIVE lock. Split into
   `staleState() (present, stale)`; removal now requires present AND stale.

   **Confidence is stated honestly:** the window is a few syscalls wide, so at
   the observed rate an A/B could not distinguish fixed from lucky within a
   practical number of runs. The committed test pins the present/absent
   distinction, not the race. The causal claim is reasoned from the code path,
   not demonstrated by reproduction.

2. **`Docs` job** failed because `a488b3b1` (this PR) added the single-node
   scheduler note directly to the GENERATED `docs/postgres-backend.md`, so
   `just docs` kept deleting it. Moved to its source entity,
   `docs-project/entities/guides/GUIDE-postgres-backend.md`; `just docs-check`
   now passes with `docs/` unchanged, confirming the generated output matches
   the hand-written text byte for byte.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One thing a reader should know: the `replace` directive in `go.mod` is a
**temporary vendored fork**, and its comment says so along with the branch it
points at. `TestPostgresQueue_SchemaPinnedDSN` is what fails if it is ever
dropped before the fix is upstream.

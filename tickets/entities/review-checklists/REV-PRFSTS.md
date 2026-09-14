---
id: REV-PRFSTS
type: review-checklist
title: 'Review: Scheduler e2e tests race TempDir cleanup; saveState outlives the assertion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -tags sqlite -shuffle=on ./internal/appbuild/... ./internal/scheduler/...` and the same without the tag: both clean. `-run TestScheduler -count=40` clean
- [x] Lint clean (`just lint`) — `golangci-lint run` → 0 issues
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links across 13850 comments
- [x] Coverage maintained (`just coverage-check`) — run locally; the change is test-only and adds no production lines, so coverage cannot fall

## Code Review

- [x] Diff reviewed line by line — one finding, fixed before commit: `lastRunAfter` originally took `*testing.T` and called `t.Helper()` despite being a poll predicate. `schedulerSettled` deliberately takes only the service, and a predicate invoked on a 20ms loop has no business holding the test handle. Signature narrowed to match
- [x] Root cause established from source, not inferred from symptoms — `recordSuccess` → `saveState` ordering at `internal/scheduler/scheduler.go:459`
- [x] The fix is load-bearing — instrumented: the new barrier blocked in 25/25 runs, no-op in 0
- [x] No production code touched — diff is confined to `internal/appbuild/scheduler_e2e_test.go`

## Reviewer Notes

**The one thing a reviewer should push on.** The race does not reproduce on
macOS: 60 runs with the barrier REMOVED still passed. So this fix is not
justified by a local red-to-green transition. It is justified by the ordering
it enforces — `saveState` provably runs after the note in every observed run —
plus three CI failures with the matching signature. If a reviewer wants
stronger evidence, the honest way to get it is to run the pre-fix test on a
Linux runner under load, not to re-run it locally.

**Scope deliberately left open.** `Scheduler.Run` still does not join an
in-flight `runDueTasks` when its context is cancelled. That means `cancel()`
followed by `<-done` is not a general barrier for production callers either —
it is only these tests that were relying on it. Fixing that is the stronger
change and is recorded as the second fix direction on BUG-PRFSTS; it is out of
scope here because it alters shutdown semantics.

**Second-run subtlety worth checking.** In `TestScheduler_EndToEnd_RepeatedRunsAccumulate (via runSchedulerTwice)`,
plain `schedulerSettled` would have been a no-op barrier: `rewindLastRun`
leaves `"tick"` in the state file, so the predicate is already true before the
second run begins. `lastRunAfter` exists specifically for that case. A reviewer
should confirm that reasoning rather than assume the two waits are equivalent.

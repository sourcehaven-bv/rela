---
id: REV-2BU0GL
type: review-checklist
title: 'Review: screenshot capture retry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`go test ./internal/docscapture/` is 27 pass / 0 skip with Chrome and the built
frontend present, and passes under `-race`. `golangci-lint` on the package
reports 0 issues; `just arch-lint` and `just comment-lint` are clean. The
package builds under all four build tags.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none outstanding. One defect was found and fixed during
the work, described below.

The first version bounded the browser launch with
`context.WithTimeout(ctx, launchTimeout)`. That is wrong, and deterministically
so: the first `chromedp.Run` is what allocates the tab, and chromedp binds the
tab's lifetime to the context it is allocated under, so `defer startCancel()`
killed the browser as soon as `launch` returned and every later action failed
with `capture: context canceled`. The bound is now applied with a timer, which
stops us waiting without owning what we waited for.

That bug is the reason `TestCapture_RetriesAgainstRealBrowser` exists. The
mocked-seam tests passed against the broken version, because a mock never
exercises the relaunch — only a real browser does.

The first version of this fix passed every local test and still failed CI with
the identical error. The retry was unreachable: `internal/docs` wired the Lua
runtime with a hardcoded 30s `buildTimeout` as the per-island cap, so gopher-lua
aborted a screenshot island at 30s regardless of the 5-minute
`screenshotBuildTimeout` the same function had chosen three lines earlier. The
island died before `Capture` could return an error worth retrying.

That is the deeper defect, and the capture-layer retry does nothing without it.
`rlua.WithTimeout` now takes the build's tier deadline. The build-wide context
still caps the total, so this does not let N islands escape the ceiling
(`TestScreenshotIslands_TotalStillBounded`).

Reverting that one argument reproduces the exact CI failure,
`lua: <string>:2: context deadline exceeded`, so the regression test pins the
real bug rather than a proxy for it.

The retry classifier is a DENY list, not an allow list. Transient failures are
open-ended (a dial timeout, a deadline mid-navigation, a tab killed under
memory pressure, a dropped websocket), so enumerating them would leave the next
new one fatally unretried. The deterministic failures are few and known, so
they are named as sentinels and everything else is retried. Worst case for an
unrecognised deterministic failure is two extra attempts before the same error.

Deliberately NOT done: raising `perCaptureTimeout`. 30s is already generous for
a localhost SPA, and it would not have helped the `could not dial` symptom at
all, which is a launch failure rather than a slow page.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| # | criterion | status | evidence |
| --- | --- | --- | --- |
| 1 | a transient failure is retried | PASS | `captureAttempts=1` mutation fails the transient cases |
| 2 | a deterministic failure is NOT retried | PASS | bypassing the classifier fails `a deterministic failure fails on the FIRST attempt` |
| 3 | the retry works against a real browser | PASS | `TestCapture_RetriesAgainstRealBrowser`; the no-reset mutation reproduces `context canceled` |
| 4 | a broken figure still fails fast | PASS | `TestCapture_UnrenderableEntity_FailsLoud` passes in 2.33s, well inside its `perCaptureTimeout - 1s` budget |
| 5 | the real pipeline still works | PASS | built `worlds-manual.md` against postgres: 16/16 screenshots, all executable assertions pass |
| 6 | a cancelled build stops promptly | PASS | `TestCapture_CancelledContextStopsRetrying` |
| 7 | a screenshot island gets the screenshot ceiling | PASS | `TestScreenshotIsland_GetsTheScreenshotCeiling`; reverting the argument reproduces the CI error |
| 8 | the ceiling still bounds the total | PASS | `TestScreenshotIslands_TotalStillBounded` stops at 2s, not 15s |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

Skipped — a build-reliability fix with no user-facing surface.

The `captureAttempts` comment records why the retry exists (cold start on a
runner shared with the Playwright suite) and why it is bounded at three rather
than unbounded. The `launchTimeout` comment records the chromedp tab-lifetime
trap, since the wrong version looks more correct than the right one.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Worth recording: this flake hit unrelated PRs and dequeued one from the merge
queue, so it read as "CI is flaky" rather than as a defect anyone owned. Both
observed failures landed on the FIRST of sixteen screenshots, which is what
identified cold start as the trigger.

The more useful lesson is the second one. I shipped a capture-layer retry that
was correct in itself, verified it against the real manual locally, and it still
failed CI identically — because the binding constraint was one layer up, in a
package I had not read. The local build passed only because it never needed the
retry. What found it was the CI timing: the job failed 34 seconds in, and three
30-second attempts cannot fit in 34 seconds, so the retry provably never ran.
Reading the elapsed time before re-reading the code was the whole diagnosis.

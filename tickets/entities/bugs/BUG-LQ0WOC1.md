---
id: BUG-LQ0WOC1
type: bug
title: TestOutputs_TriggerFinalCloseOnly misses the close when a mark arrives mid-round
description: "TestOutputs_TriggerFinalCloseOnly in internal/appbuild failed on PR 2093 (job 126664977178): realtime_runas_internal_test.go:544 Condition never satisfied after 20.98s; passed on rerun. Likely cause: startOutputs skips a wake while the last outputs round runs (outputsBusy), so the mark for the final move waits for the minute tick, after the 20s window. Fix: rerun a busy round when a wake arrived during it, or have the test wait for the first round before the move."
priority: low
status: backlog
---

## Description

`internal/appbuild/realtime_runas_internal_test.go`, test
`TestOutputs_TriggerFinalCloseOnly`, failed in CI on an unrelated PR (#2093,
frontend only), job
https://github.com/sourcehaven-bv/rela/actions/runs/41856088118/job/126664977178.
It passed on rerun.

```
realtime_runas_internal_test.go:544:
    Error:      Condition never satisfied
    Test:       TestOutputs_TriggerFinalCloseOnly
--- FAIL: TestOutputs_TriggerFinalCloseOnly (20.98s)
```

Line 544 is the second `require.Eventually`, which waits 20s for the channel to
close and for the second mark to record its result. The log shows two trigger
rounds that each posted a mark, and one session outputs round with `ran=1`. No
second outputs round ran.

## Likely cause

`marksWake` starts an outputs round when a trigger round wrote marks.
`startOutputs` does nothing while the last round still runs (`outputsBusy`).
When the move to `cold` is posted while the round for `hot` still runs, that
round does not pick up the new mark, and the mark waits for the minute tick. The
tick comes after the test's 20s window.

## Fix options

- Have a busy outputs round run again when a wake arrived during it, so a
mark written mid-round is not left for the tick.
- Or, in the test, wait for the first outputs round to finish before the move
to `cold`.

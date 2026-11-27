---
id: BUG-LQ0WOC1
type: bug
title: TestOutputs_TriggerFinalCloseOnly misses the close when a mark arrives mid-round
description: 'The realtime worker dropped an outputs wake that arrived while an outputs round ran: startOutputs returned false when outputsBusy was set and recorded nothing. A mark written mid-round then waited for the minute tick. TestOutputs_TriggerFinalCloseOnly failed in CI (PR 2093, job 126664977178) when the move to cold landed during the round for hot. Fixed with a dirty flag: each wake sets it and the round goroutine reruns while it is set.'
priority: low
why1: 'The second outputs mark waited for the minute tick because startOutputs skipped the wake from the trigger round that wrote it: an outputs round for the first mark was still running.'
why2: startOutputs used one busy flag for both mutual exclusion and wake delivery. A wake that lost the CompareAndSwap was discarded and nothing reran the round after it ended.
why3: The design assumed a running round would pick up new marks, but a round reads the pending marks once at its start, so a mark written after that read is invisible to it.
why4: The existing unit test TestWorker_OutputsRoundBesideTheLoop pinned that a second start is skipped, which encoded the drop as intended behaviour; no test covered a wake during a round.
why5: Coalescing wake signals for a single-flight worker is a known pattern (busy plus dirty flag) that was not applied; integration tests relied on timing windows instead of forcing the interleaving.
prevention: TestWorker_OutputsWakeDuringRoundReruns pins the rerun with a gated round. TestOutputs_TriggerFinalCloseOnly now holds the first round until the second wake reached the busy runner, so it exercises the mid-round path every run instead of by chance.
started: "2026-11-27"
completed: "2026-11-27"
status: done
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

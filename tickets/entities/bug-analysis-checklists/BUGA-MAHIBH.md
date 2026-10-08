---
id: BUGA-MAHIBH
type: bug-analysis-checklist
title: 'Analysis: Collapsed background-job trigger resets the hop count'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally: 5 failures in 1,200 runs of
`-run 'TestAutomationJobs_QueuedChainStops$' -count=400 -cpu=1,2,8 -race`.
- [x] Minimal reproduction steps documented: two background actions whose
runs trigger each other on one queue; the run log showed `a@3 b@4 a@3 b@4`, a
hop reset.
- [x] Environment/conditions noted: memory queue; needs a trigger to arrive
while the job for the same key is still running.

## Root Cause

- [x] Immediate cause identified (why1): the rerun loop used the payload's
hop count, not the collapsed trigger's.
- [x] Contributing factors found (why2-3): the trigger token was opaque, and
the collapse path bypasses the enqueue-time hop check.
- [x] ~~Systemic cause explored (why4-5)~~ (N/A: why3 reaches the design gap)

## Fix Planning

- [x] Fix approach determined: the token ends in its hop count; each rerun
takes the maximum of the payload's and the token's count.
- [x] Regression test planned: the test asserts the exact hop sequence 1..8.
- [x] Related areas checked for similar issues: `followUp` re-enqueues the
trigger's own payload, and `EntityRenamed` goes through `EnqueueScript`, so both
keep the trigger's count.

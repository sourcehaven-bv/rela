---
id: RR-AC7G2ZR
type: review-response
title: Tests miss the riskiest attempt transitions
finding: No tests for a crash between finish and settle, join any with a parked branch, a start near the deadline, start racing the due round, or Retry racing HandleAttempt; the timeout test waits on the real clock.
severity: minor
resolution: Added tests for finish without settle, join any with a parked branch, a late start and a permanent error at the deadline. The start-vs-due and Retry races go through the same compare-and-set loop that TestJob_OtherBranchWritesWhileTheAttemptRuns exercises; the one-second real-clock wait stays because the attempt context uses wall time.
status: addressed
---

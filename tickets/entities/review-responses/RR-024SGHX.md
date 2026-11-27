---
id: RR-024SGHX
type: review-response
title: No test reaches the recheck after releasing outputsBusy
finding: The path after outputsBusy.Store(false) that takes the runner back for a wake in the release window is the subtle part of the fix and had no test.
severity: minor
resolution: 'Added TestWorker_OutputsNoLostWake: 8 goroutines x 200 wakes x 20 runs, asserting a round started after the last wake. Replacing the recheck with a plain return makes it fail.'
status: addressed
---

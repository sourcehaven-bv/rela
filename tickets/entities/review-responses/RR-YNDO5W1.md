---
id: RR-YNDO5W1
type: review-response
title: TestWorker_OutputsRoundBesideTheLoop doc says a second start is skipped
finding: With the dirty flag a second start is coalesced into a rerun. runs == 1 holds only because the canceled lead drops the rerun.
severity: minor
resolution: 'Reworded the test doc and assertion messages: a second start starts no second goroutine and a canceled lead runs no rerun.'
status: addressed
---

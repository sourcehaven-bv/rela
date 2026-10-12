---
id: RR-J6T9MV
type: review-response
title: Skipped or unfinished targets wait for another event
finding: 'Cranky review: dirty was cleared before a pass, so a target skipped by the budget or a replay that did not finish kept its old digest until an unrelated event arrived. A change could go unreported until the timeout.'
severity: significant
resolution: pass returns incomplete; the loop keeps dirty set when a pass was incomplete, including the first pass. TestWatch_UnfinishedReplayRetried pins it, with a mutation check.
status: addressed
---

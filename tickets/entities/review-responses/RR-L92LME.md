---
id: RR-L92LME
type: review-response
title: Watch tests miss budget, rotation and retry paths
finding: 'Cranky review: budget and replay timeout were constants, and no test reached rotation or the retry of unfinished replays.'
severity: significant
resolution: budget and replayTimeout are watchHandler fields. Added TestWatchRun_BudgetRotates, TestWatch_UnfinishedReplayRetried and TestWatch_PassBoundedByTimeout, each mutation-checked.
status: addressed
---

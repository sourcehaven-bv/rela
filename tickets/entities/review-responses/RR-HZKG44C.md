---
id: RR-HZKG44C
type: review-response
title: 'Change: a sibling branch write makes a valid step stale'
finding: The run revision covers every branch, so any write to a sibling (claim, mark, arrival) lost the step's compare-and-set and Change answered ErrStale after the subject write had landed.
severity: significant
resolution: Change reads the run again and retries up to five times while the step's branch is still at gen. TestFork_ChangeRetriesAfterASiblingWrite injects a sibling claim before the move.
status: addressed
---

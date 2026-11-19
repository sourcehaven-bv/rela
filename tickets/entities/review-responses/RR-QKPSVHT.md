---
id: RR-QKPSVHT
type: review-response
title: Zero instance trusted the state of the mark
finding: The helper used the state of the mark for a run with no stored instance instead of the initial state.
severity: minor
resolution: FailedJobs uses flow.Read, which maps a zero instance to the initial state, as place() does.
status: addressed
---

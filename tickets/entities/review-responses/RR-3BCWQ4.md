---
id: RR-3BCWQ4
type: review-response
title: Test double reimplements the success path instead of calling it (root cause of the two surviving mutants)
finding: newRetryTestScheduler's override delegates to the real recordFailure but hand-copies the success bookkeeping (state.Tasks write + ladder clears), duplicating scheduler.go:283-289. Assertions therefore validate the copy. This duplication is the shared root cause of RR-F6182G and RR-QOSJZ5. Extracting a recordSuccess method called by both doExecuteTask and the double kills both mutants.
severity: significant
resolution: recordSuccess extracted and called by both doExecuteTask and the test double, so the success path is no longer duplicated. This is what makes the RR-F6182G and RR-QOSJZ5 mutants fail.
status: addressed
---

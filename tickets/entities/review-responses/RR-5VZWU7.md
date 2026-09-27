---
id: RR-5VZWU7
type: review-response
title: A child job executes after its run has ended, so a retried fan-out can deliver twice
finding: runChildJob ignores ExtendLease's outcome and always runs the subject. When a run is reaped while a child retry is still pending (neoq backoff plus a hung attempt can exceed runningLease), the retry run re-enqueues the unsucceeded subject under a new key and both copies execute.
severity: significant
resolution: 'ExtendLease is replaced by Store.StartChild: a claim that also extends the lease. It returns false when the run has ended, and runChildJob then skips the subject with a WARN. Pinned by conformance StartChildRefusesSettledSubjectOrEndedRun and TestForEach_ChildOfAbandonedRunDoesNotExecute.'
status: addressed
---

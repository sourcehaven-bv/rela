---
id: RR-UUWC92
type: review-response
title: An unguarded RecordFailure can clobber a concurrent success and pin a healthy task to the ladder
finding: 'RecordSuccess is conditional (WHERE last_run < $new) but RecordFailure has no conflict rule — an asymmetry that was a gap rather than a decision. Concrete failure: node A succeeds at T and clears the ladder; node B, which STARTED the same task at T-1 and failed, then writes failures=1, next_retry=T+5m. The task is now ladder-driven despite having just succeeded, and the retry branch suppresses its normal schedule. A merely-lost increment would be tolerable (the ladder only needs to back off roughly), but clobbering a success is not. Fix: guard RecordFailure with the same WHERE last_run < $start predicate, expressing ''only if no successful run has started since mine'', and pin it in the conformance suite.'
severity: significant
resolution: 'Both RecordFailure and SetNextRetry now carry the run''s START time and are guarded on the same WHERE last_run < $start predicate RecordSuccess uses — ''only if no successful run has started since mine''. The two write paths are therefore symmetric, which is the property that should have been designed in rather than left as a gap. Acceptance criterion 4 pins it: a RecordFailure whose start predates a recorded success is a no-op.'
status: addressed
---

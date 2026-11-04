---
id: RR-55P6KW3
type: review-response
title: Some served-record paths have no test
finding: No test drives a record while syncing, an inbox sub deferred off the grid, a dueOnly overflow, or the 304 skip. TestLive_ServedAfterCommitFrames comment claimed a slow read that does not happen.
severity: minor
reason: The comment now says the record is injected. The other paths reuse the pending and recheck machinery that live_baseline_test.go, live_inbox_test.go and live_busy_test.go already cover, and the v1 entity GET never answers 304 when leases are wired.
status: deferred
---

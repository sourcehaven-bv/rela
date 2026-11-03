---
id: RR-UU9F7G
type: review-response
title: PoC flow test skips the re-entry and since checks
finding: 'internal/dataentry/busy_rows_test.go:409-427 stops at plan-review. It never posts approve, so it does not show that the session going back to planning is busy again with a new `since`, and it never asserts `since` at all. It also does not check the `new` state before `go`, because the channel does not exist yet. Fix: post approve and assert one Planning entry whose Since is later than the first claim. Assert Since equals the claim''s time on the first planning step (use the app clock).'
severity: minor
resolution: The PoC test uses a fake clock, asserts since equals the claim time, then posts approve and asserts the session is busy in Planning again with since equal to the approve time.
status: addressed
---

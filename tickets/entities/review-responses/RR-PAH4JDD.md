---
id: RR-PAH4JDD
type: review-response
title: Two clocks in data entry
finding: The router stamps nowFunc while appbuild uses WithClock; a test that pins one does not pin the other.
severity: minor
reason: Both are time.Now in production; unifying the data-entry clock with appbuild.WithClock is wider than this ticket.
status: deferred
---

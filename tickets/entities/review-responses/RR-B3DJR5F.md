---
id: RR-B3DJR5F
type: review-response
title: Short-id length uses COUNT over the store
finding: mintShortID counts every row to pick the id length; COUNT is O(n) on pg and sqlite, invisible to the row counter.
severity: minor
reason: Index-only count once per create; replacing it changes the id-length policy. Out of scope for this ticket.
status: deferred
---

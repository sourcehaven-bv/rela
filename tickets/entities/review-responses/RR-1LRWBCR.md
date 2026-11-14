---
id: RR-1LRWBCR
type: review-response
title: Datetime comparison may plan updates every run
finding: A stored time.Time and source text in another spelling never compare equal.
severity: minor
reason: Datetime properties are stored as the text the manager validated on fs and memory stores. Instant comparison needs a per-backend check; follow-up.
status: deferred
---

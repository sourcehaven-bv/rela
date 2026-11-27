---
id: RR-0R5FMHC
type: review-response
title: Manual list reads skip pushdown
finding: manual_order=1 turns off the store pushdown and a move rereads the whole gated list.
severity: minor
reason: 'No new read path: it is the Go fallback every list takes when its condition does not lower, and a grouped list already reads all rows. The unpaged-read archguard passes. Pushing stored ids into the query is follow-up work.'
status: deferred
---

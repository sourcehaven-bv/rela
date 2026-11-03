---
id: RR-M79DDK5
type: review-response
title: pg does not refuse a schema newer than the binary, so previous can be stale
finding: A pre-0040 binary on a migrated database moves sessions without writing session_previous; the column then shows an earlier move. Cause has the same flaw.
severity: minor
reason: A deployment policy for every pg migration, outside this ticket. Backlog ticket TKT-D3BQJ5J (depends on TKT-56TPOTE).
status: deferred
---

---
id: RR-3YI72U9
type: review-response
title: Gone guard is per run so deletes can accumulate
finding: 'A source dropping up to max_gone rows per run could delete thousands of rows per day with on_gone: delete.'
severity: minor
resolution: 'With on_gone: delete the guard also counts deletes of the last 24 hours (kept in the ledger) against max_gone. TestRun_DeleteWindow; threat model updated.'
status: addressed
---

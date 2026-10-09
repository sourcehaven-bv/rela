---
id: RR-N2MXHR
type: review-response
title: Each tick still probes every settled row
finding: With 50,000 clean rows the candidate query takes about 185 ms per tick, because it probes the version index for every row; develop stopped after Batch rows.
severity: minor
reason: 185 ms per 5-minute tick at 50,000 entities is acceptable; atlas has 825. Reducing it needs synchronous version writers (rename, delete, restore, purge) to clear the live hash so the gate becomes content_hash IS NULL with a partial index; that is a separate change.
status: deferred
---

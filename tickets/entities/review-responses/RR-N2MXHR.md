---
id: RR-N2MXHR
type: review-response
title: Each tick still probes every settled row
finding: With 50,000 clean rows the candidate query takes about 185 ms per tick, because it probes the version index for every row; develop stopped after Batch rows.
severity: minor
resolution: 'Follow-up branch perf/sweep-dirty-index (TASK-Y73Y9 in Atlas): triggers on the version tables keep a stored hash equal to the latest version''s, the write-back checks the latest version, and the sweep selects content_hash IS NULL through a partial index (0.09 ms at 5,000 swept entities).'
reason: 185 ms per 5-minute tick at 50,000 entities is acceptable; atlas has 825. Reducing it needs synchronous version writers (rename, delete, restore, purge) to clear the live hash so the gate becomes content_hash IS NULL with a partial index; that is a separate change.
status: addressed
---

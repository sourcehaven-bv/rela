---
id: RR-DI92SQY
type: review-response
title: A create that fails without an answer is sent again and duplicates the row
finding: pushCreate retried a POST that failed on a broken connection or a 5xx. Such a create may have reached the remote; the next run would POST again and make a second remote row, which the pull then copies back as a second local row.
severity: significant
resolution: A POST that fails without a clear refusal holds the row as ReasonUncertain. Local edits do not release it; only a resolve to local does, after a person checks the remote. TestPush_UncertainCreateIsNotRepeated. Documented in remote-sync.md.
status: addressed
---

---
id: RR-C0EH2OA
type: review-response
title: The current-suggestion fallback rereads queries that were already read whole
finding: queryCandidates fell back to a full read whenever it had want.First rows and Keep was missing, also for queries with sort:, free text or no type, which firstN does not cut. A stale current_entity then caused a second full read.
severity: significant
resolution: executeQueryCut reports whether firstN cut the read short; the fallback runs only then. TestNextAction_StaleCurrentOnAWholeReadReadsOnce pins one read for a sorted query with a stale current entity. A bounded lookup of the one id is TKT-R98U9GZ.
status: addressed
---

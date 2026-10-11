---
id: RR-FBU79L
type: review-response
title: Fallback count loads and yields every row
finding: Without pushdown CountEntities drained ListEntities and yielded each redacted row
severity: significant
resolution: 'The fallback now counts len(Filter(listGatedThenRanked)) directly: the rows the list fallback keeps.'
status: addressed
---

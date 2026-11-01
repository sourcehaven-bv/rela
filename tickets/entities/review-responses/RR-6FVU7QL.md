---
id: RR-6FVU7QL
type: review-response
title: 'Limit pushdown is wrong with a sort: clause'
finding: executeQueryPrefiltered sorts with sortEntitiesMulti after the relevance cap (queryservice.go:197-199). Pushing limit=50 takes the top 50 by relevance and then sorts, a different set than sorting the top 1000. Exclude sq.HasSort() from pushdown.
severity: significant
resolution: 'Plan changed: no pushdown when the query has a sort: clause; test TestSearchLimitPushdown covers it.'
status: addressed
---

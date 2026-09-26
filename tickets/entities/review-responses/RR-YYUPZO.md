---
id: RR-YYUPZO
type: review-response
title: Limit applied before the face filter
finding: SearchVisibleFields applied q.Limit before gatedSearcher's face filter, so face-denied hits consumed the limit and the godoc overclaimed.
severity: significant
resolution: The inner search runs with Limit 0; gatedSearcher applies the face filter and then the limit.
status: addressed
---

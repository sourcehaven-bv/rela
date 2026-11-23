---
id: RR-8B4FN7V
type: review-response
title: A count card sent with limit=1 would report total 1 for free text
finding: The plan's count-card optimization sends limit=1. freeTextLimit lowers the relevance cap to the caller's limit when the query has no property filter or sort, so a free-text count card would read one hit and report total 1.
severity: significant
resolution: 'Addressed: count cards keep the request without limit. Only breakdown and table query cards page.'
status: addressed
---

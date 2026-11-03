---
id: RR-YFTZ31
type: review-response
title: Sort fallback retries on any property_not_visible
finding: withSortFallback retried without sort on any property_not_visible error, including a refused filter, so it made a second request it could not fix.
severity: minor
resolution: 'The 400 body carries use (filter or sort) and property, from a typed propertyNotVisibleError. withSortFallback retries only when use is sort. Tests in entitiesSortFallback.test.ts: a refused filter is not retried, and a refusal without use is not retried. Goldens regenerated with the new fields.'
status: addressed
---

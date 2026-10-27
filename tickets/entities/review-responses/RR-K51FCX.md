---
id: RR-K51FCX
type: review-response
title: Result is unbounded
finding: TaggedEntities returns every row for a tag name with no paging.
severity: minor
resolution: One row per tagged face is the size of the baseline the caller made. The guide now says the list is not paged and advises one tag name per baseline.
status: addressed
---

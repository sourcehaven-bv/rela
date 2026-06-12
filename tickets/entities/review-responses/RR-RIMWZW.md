---
id: RR-RIMWZW
type: review-response
title: canonicalListParams k=v join could collide on values containing & or =
finding: The cache-key serializer joined sorted pairs as `k=v&...`, so two distinct filter sets could collapse to the same key when a filter value contained & or = (both legal in a contains-filter), serving the wrong cached list — the same false-negative class utils/filters.ts:stringifyFilterQuery documents and avoids.
severity: significant
resolution: Switched to JSON.stringify of sorted [key,value] pairs (collision-proof, also future-proofs array/object values). Added a regression test asserting filter[a]=x,filter[b]=y does not collide with filter[a]=x&filter[b]=y.
status: addressed
---

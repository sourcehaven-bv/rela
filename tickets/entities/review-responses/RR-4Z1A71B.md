---
id: RR-4Z1A71B
type: review-response
title: Missing NUL separator after the query index shape in the hash
finding: queryIndexName wrote queryIndexShape and the entity type with no separator, unlike the other parts.
severity: nit
resolution: A NUL byte now separates them.
status: addressed
---

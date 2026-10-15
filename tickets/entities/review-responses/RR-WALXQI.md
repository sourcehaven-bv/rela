---
id: RR-WALXQI
type: review-response
title: List values deduplicated as whole lists
finding: JSON.stringify of the whole array made [a,b] and [a] show a twice.
severity: significant
resolution: useRelatedCardValues flattens list values to elements before deduplicating and resolves the element widget; Vitest covers it; the guide says so.
status: addressed
---

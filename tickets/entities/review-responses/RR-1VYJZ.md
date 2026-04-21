---
id: RR-1VYJZ
type: review-response
title: Runtime.cache field typed as concrete *Cache (round 2)
finding: The cache field on Runtime is typed as concrete *lua.Cache. Adding a disk-backed variant later (TKT-135Q reuse or other) means touching the field type and every WithCache call site. Typing the field as an interface at the consumer makes the cutover a drop-in.
severity: minor
resolution: Defined unexported cacheStore interface with get/set/delete methods; Runtime.cache is now typed as cacheStore. *Cache still satisfies it, WithCache still takes *Cache publicly, but future alternate backends slot in without rewiring.
status: addressed
---

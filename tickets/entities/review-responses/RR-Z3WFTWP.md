---
id: RR-Z3WFTWP
type: review-response
title: Record types are fetched on every load
finding: Each load calls getRecordTypes before the title lookups.
severity: nit
reason: One small config request per card load, bounded by the 8-owner cap; the schema store does not hold record types today. Not worth a new cache.
status: wont-fix
---

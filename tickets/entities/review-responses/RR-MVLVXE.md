---
id: RR-MVLVXE
type: review-response
title: Flush ignores refetchOnWindowFocus false
finding: The visibility flush refreshes every active query, and the global default is now false, so a per-query opt-out cannot be seen.
severity: minor
resolution: The flush calls cache.refresh, which fetches only stale entries; the one opt-out query (configure snapshot) has staleTime Infinity. Documented in visibleRefetch.ts that staleTime Infinity is the opt-out.
status: addressed
---

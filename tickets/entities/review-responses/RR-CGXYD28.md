---
id: RR-CGXYD28
type: review-response
title: pg search fails on history rows
finding: The instant CTE sets search_title NULL for version rows; the search distance is NULL and the scan fails.
severity: critical
resolution: The pgstore snapshot recomputes search_title from the version's properties the way the write path does, and search reads go through the instant seam. Pinned by RunInstantTests/SearchFindsTheTextAtTheInstant.
status: addressed
---

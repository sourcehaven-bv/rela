---
id: RR-TTHRSL
type: review-response
title: Budget test must count the fallback's real reads
finding: Counting one call hides the generic fallback's reads on fs/memory.
severity: minor
resolution: Counting forwards a native Rollup as one read and runs the generic fallback through itself so each inner read is counted; the budget test asserts the count is equal at 10 and 50 rows.
status: addressed
---

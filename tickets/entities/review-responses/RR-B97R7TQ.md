---
id: RR-B97R7TQ
type: review-response
title: Later keyset pages re-read the rows before the cursor
finding: A GiST distance scan cannot start at the cursor so each later page of a gated search reads past every earlier row again.
severity: minor
resolution: Documented on visiblePages. A limited search whose first page fills the limit now stops there instead of reading a second page (TestSearchVisible_StopsAtLimit).
status: addressed
---

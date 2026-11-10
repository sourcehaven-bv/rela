---
id: RR-5N62IL5
type: review-response
title: Paging overflows for a huge page
finding: RunList and Tasks computed lo := min((page-1)*perPage, n). page and per_page come from HTTP and MCP callers, so a huge page overflowed to a negative lo and the slice panicked.
severity: significant
resolution: Both lists page through pageRange. It returns an empty page when page-1 > n/perPage, so the product it then computes is at most n. The HTTP and MCP handlers only pass the values through. TestPageRange covers math.MaxInt page and page size; TestFlowControl_RunList covers them end to end.
status: addressed
---

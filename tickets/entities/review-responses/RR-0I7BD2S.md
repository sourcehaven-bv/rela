---
id: RR-0I7BD2S
type: review-response
title: FirstEqual pages two rows at a time
finding: Rows the store admits but the Go match drops (lists holding the value or hidden properties) cost one round trip per two rows; the security review also noted the latency could hint at hidden values.
severity: minor
resolution: holderScan has a page size; FirstEqual reads 64 header rows per page so such rows cost one round trip per 64.
status: addressed
---

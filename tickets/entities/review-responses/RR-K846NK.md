---
id: RR-K846NK
type: review-response
title: Trace lost events on quick exit
finding: Only a 250 ms timer flushed the buffer.
severity: nit
resolution: Exit flushes when a goroutine's outermost traced call returns.
status: addressed
---

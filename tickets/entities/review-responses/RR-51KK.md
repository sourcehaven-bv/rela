---
id: RR-51KK
type: review-response
title: 'S4: No test asserted that SSE response lacks Access-Control-Allow-* headers'
finding: watcher.go removed the CORS reflection on the SSE handler, but TestHandleSSEHeaders only checked Content-Type and Cache-Control. A future commit could trivially reintroduce the Access-Control-Allow-Origin reflection without any test failing — exactly the bug that triggered this whole ticket.
severity: significant
resolution: Added regression assertions to TestHandleSSEHeaders that fail if either Access-Control-Allow-Origin or Access-Control-Allow-Credentials is non-empty on the SSE response. Comment cites the original bug so the next reviewer understands why the assertion exists.
status: addressed
---

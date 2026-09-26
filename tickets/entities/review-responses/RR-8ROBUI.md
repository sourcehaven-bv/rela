---
id: RR-8ROBUI
type: review-response
title: Concurrent large MCP HTTP bodies can exhaust memory
finding: Raising MaxRequestBodyBytes to ~22 MiB lets any authenticated caller send many concurrent large requests; the SDK buffers each body and the tool holds several copies while waiting on the write lock, before the update preflight runs.
severity: significant
resolution: largeRequestGate in HTTPHandler admits at most two requests over 4 MiB (or with no Content-Length) before the SDK reads the body; others wait holding only a connection. TestLargeRequestGate.
status: addressed
---

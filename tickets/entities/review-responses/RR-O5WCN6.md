---
id: RR-O5WCN6
type: review-response
title: No concurrency tests for the service
finding: No test exercises the limiter, ephemeral store, Local mark/Take, timer re-arm or record-vs-reload concurrently, so the race detector cannot catch anything there.
severity: significant
status: open
---

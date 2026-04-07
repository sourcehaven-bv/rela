---
id: RR-CSTD
type: review-response
title: 30s timeout under global write lock is a DoS
finding: Default 30s timeout combined with required write lock means a single bad script freezes the entire server for 30 seconds. Reduce to ~5s and explicitly set via WithTimeout.
severity: critical
resolution: Explicit 5s timeout via lua.WithTimeout(5*time.Second). Documented that this is tighter than CLI default because of the write lock.
status: addressed
---

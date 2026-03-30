---
finding: Global rng variable is shared across goroutines without mutex. rand.Rand is not thread-safe. Tests running in parallel will have data races.
id: RR-e0xt
resolution: Added sync.Mutex (rngMu) and wrapped all RNG access with Lock/Unlock for thread safety.
severity: critical
status: addressed
title: 'Race condition: global RNG without synchronization'
type: review-response
---

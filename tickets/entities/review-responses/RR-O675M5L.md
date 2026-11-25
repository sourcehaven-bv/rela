---
id: RR-O675M5L
type: review-response
title: Lock held across main-thread Register call
finding: swap ran under q.mu while Register waits for the main thread whose close handler takes q.mu
severity: significant
resolution: Register and Unregister run outside q.mu under a separate swapMu that nothing on the main thread takes.
status: addressed
---

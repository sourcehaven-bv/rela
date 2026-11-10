---
id: RR-L2DYQH7
type: review-response
title: Eviction teardown blocks another tenant's request
finding: closeAll and the last release ran the resident's Close inline, including a relay shutdown of up to 5s.
severity: significant
resolution: Evicted residents close on background goroutines tracked by a WaitGroup; Registry.Close waits for them.
status: addressed
---

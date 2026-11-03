---
id: RR-3KCDVM
type: review-response
title: Desktop closed the store without waiting for the scheduler
finding: releaseLoadedProject waited only for the lease watcher; every project now runs a scheduler.
severity: significant
resolution: startBackground merges the scheduler and lease done channels; release waits for both before Close.
status: addressed
---

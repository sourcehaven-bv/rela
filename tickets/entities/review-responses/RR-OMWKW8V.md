---
id: RR-OMWKW8V
type: review-response
title: Retry timer callback can outlive Close
finding: A retry timer whose callback had already fired when Close stopped the timers was not waited for, so Close could return while the callback still ran and the goroutine leak test would flake.
severity: significant
resolution: 'Addressed: retryLater adds the pending retry to the workers WaitGroup; the callback or Close (when Stop wins) calls Done.'
status: addressed
---

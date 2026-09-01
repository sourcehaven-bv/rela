---
id: RR-3F8F4F
type: review-response
title: foldGantt was unbounded recursion on data-controlled depth (process-killing stack overflow)
finding: The fold recursed once per containment level with no bound. MaxDepth applies only to emission, and a 50k-deep chain of `contains` edges is valid data — a goroutine stack overflow panics the whole process, not the request.
severity: significant
resolution: foldGantt rewritten as an iterative post-order walk on an explicit stack (frame{id,next}); reachability marking and rolled-span contribution preserved, all gantt tests green. The emit walk keeps recursion but is bounded by the depth cap and node budget.
status: addressed
---

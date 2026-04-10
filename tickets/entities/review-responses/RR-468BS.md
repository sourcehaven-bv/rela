---
id: RR-468BS
type: review-response
title: context.Background in task execution ignores shutdown signal
finding: Tasks get context.Background() so they ignore parent cancellation. Graceful shutdown could take up to 5min for a task that just started.
severity: significant
resolution: Tasks now receive parent ctx from Run(). doExecuteTask checks ctx.Err() before executing and skips if shutting down
status: addressed
---

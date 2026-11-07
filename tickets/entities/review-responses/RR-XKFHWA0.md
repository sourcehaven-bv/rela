---
id: RR-XKFHWA0
type: review-response
title: goleak exemption for the cron runner is broader than the leak
finding: IgnoreTopFunction hides every robfig cron goroutine.
severity: minor
resolution: The goroutine test now counts cron runners via runtime.Stack and asserts at most one per world.
status: addressed
---

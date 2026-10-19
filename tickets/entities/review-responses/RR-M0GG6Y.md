---
id: RR-M0GG6Y
type: review-response
title: Concurrent second Close returns before handlers finish
finding: A second Close while the first is waiting returned nil at once.
severity: minor
resolution: The first Close creates a drained channel; every Close waits on it with its own ctx.
status: addressed
---

---
id: RR-YTXO4R
type: review-response
title: fs tier multi-process refresh
finding: fs has no process lock; CLI, server and scheduler can refresh at once.
severity: significant
resolution: 'Plan R4: sealed store only on sqlite and postgres (plus desktop keychain); fs not configured; tier table in docs.'
status: addressed
---

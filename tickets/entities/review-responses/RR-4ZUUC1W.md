---
id: RR-4ZUUC1W
type: review-response
title: InlineQueue breaks the Enqueue contract and can deadlock inside memstore Tx
finding: jobs.go:298-308 Enqueue returns on accept; jobs.md forbids running before commit; memstore Tx mutex deadlocks on re-entrant writes. Background automations already run in the foreground without WithBackgroundAutomationJobs.
severity: significant
resolution: Dropped InlineQueue and WithJobQueue. Tests rely on foreground delivery of background actions; documented.
status: addressed
---

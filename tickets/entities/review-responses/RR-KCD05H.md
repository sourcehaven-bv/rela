---
id: RR-KCD05H
type: review-response
title: 'Design: one-shot processes lose jobs'
finding: CLI commands start a memory queue and exit; on postgres they also claim shared jobs.
severity: critical
resolution: 'Delivery is an entry-point property: WithBackgroundAutomationJobs (rela-server, desktop) uses the queue; every other assembly runs the action in the foreground after the cascade through the same handler. Pre-existing postgres CLI drop filed as BUG-DNJVS3.'
status: addressed
---

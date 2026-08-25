---
id: RR-3HLCZB
type: review-response
title: 'Interface over-built: Get has no consumer and All() is unbounded on a shared database'
finding: Two shape problems. (1) Get(ctx, task) has no caller — runDueTasks iterates the configured task list and needs every task's state per tick, so it uses the bulk read; an interface method with no consumer violates the consumer-defined-minimum rule. Drop it until a CLI status command needs it. (2) All() invites an unbounded read of the whole table. Scope it to the configured names — Load(ctx, tasks []string) — which is safer on a shared database and, as a side effect, makes orphaned rows simply never read, dissolving most of the prune problem in RR-JKT6PZ.
severity: minor
status: open
---

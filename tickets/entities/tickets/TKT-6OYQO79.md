---
id: TKT-6OYQO79
type: ticket
title: 'Declarative metric capture: schedule task that appends a dashboard card value as a record'
kind: enhancement
priority: low
effort: m
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

Follow-up of TKT-RK810V7. A schedule task that captures a metric without Lua: it
evaluates a named dashboard card (or a query) under the task's `run_as` identity
and appends the value as one `measurement` row to a metric owner.

Open questions: which card displays can be captured (count, ratio,
sum/avg/min/max), how the task names the owner and the field, and how a failed
evaluation is reported. The value is computed under `run_as`, so the same
read-rule guidance applies as for the Lua capture pattern.

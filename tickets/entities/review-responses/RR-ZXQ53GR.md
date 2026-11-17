---
id: RR-ZXQ53GR
type: review-response
title: Missing binding tests for views, feeds, gantt, mail and CLI
finding: These surfaces call the shared helpers but have no test of their own.
severity: minor
reason: They go through parseWhereAt and BindDateVariables, which are tested; per-surface tests can follow.
status: deferred
---

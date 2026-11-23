---
id: TKT-5SXPBL1
type: ticket
title: Push the list shapes the pushdown declines into the store
kind: chore
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-C5H506O. `listPipeline.scopedSortedEntitiesScoped`
(`internal/dataentry/list_pipeline.go`) reads every readable row of the type
through `scanHeaders` when `planListPushdown` declines the request: relation
filters, a condition or query scope that does not lower exactly, a Go sort, a
page beyond the pushdown. The same read serves `_position`, export, the gantt
scoped arm, next action and nav status. Lower the remaining shapes into
`store.GraphQuery` so those requests read a page, then shrink or remove its
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`.

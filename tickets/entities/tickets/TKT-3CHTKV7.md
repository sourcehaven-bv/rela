---
id: TKT-3CHTKV7
type: ticket
title: Count nav status markers in the store
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up of TKT-D3RAB0E. `navStatusRows` (`internal/dataentry/nav_status.go`)
reads every row of the type through `scopedSortedEntitiesScoped` and judges the
marker conditions in Go, so its rows grow with the store.
`TestQueryBudget_NavStatusIsSizeIndependent` and
`TestQueryBudget_NavStatusRuleOnlyIsSizeIndependent` are still calls-only.

Count each marker with `store.CountMatched` on the gated query when its
condition lowers exactly (as `planSearch` in `internal/dataentry/searchplan.go`
decides for `/_search`), and keep the Go path otherwise. The markers are
aggregates: the count must run on the row-gated query (CLAUDE.md, "Aggregates
computed over the graph gate BEFORE they fold"). Convert both budget tests to
rows checks.

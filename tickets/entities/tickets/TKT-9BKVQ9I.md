---
id: TKT-9BKVQ9I
type: ticket
title: Narrow the list_entities Go path for filters that do not lower
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

After TKT-Z099FGY, MCP `list_entities` answers in the store every filter that
`queryplan.LowerCondition` lowers. The rest (ordered comparisons, `or`, `not`
over a non-traversal, equalities on a property hidden on some rows, and a
traversal the gate refuses) still read the whole readable type with bodies and
filter in Go (`internal/mcp/tools_entity.go#listPage.filterInGo`, a
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`).

The response carries an exact total, so every candidate row must be judged. Two
narrowings remain possible:

- Push the lowerable conjuncts of the AND spine as a superset pre-filter
(`queryplan.ConditionPrefilters`) and judge only the remainder in Go.
- Read content-free headers instead of whole entities.

Neither makes the read scale-free. Decide whether either is worth it, or whether
the total should become optional for such filters.

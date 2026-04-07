---
id: TKT-KP3I
type: ticket
title: Add date comparison operators and $today substitution to v1 list filters
kind: enhancement
priority: medium
effort: s
status: done
---

## Description\n\nExtend the data-entry v1 list filter API with two missing capabilities:\n\n1. **Comparison operators**: `lt`, `lte`, `gt`, `gte` with date-aware comparison. Currently `applyV1Filters` only supports `eq`/`ne`/`contains`/`in`, so date range queries like `due_date <= 2026-04-07` were impossible.\n\n2. **Variable substitution**: `$today`, `$tomorrow`, `$yesterday` resolve to current dates server-side. Without this, filter values are static strings — a list configured with `value: 2026-04-07` becomes stale the next day.\n\nUnlocks dynamic filtered lists like \"Overdue Tasks\" defined entirely in `data-entry.yaml`.\n\n## Implementation\n\n- `internal/dataentry/helpers.go` — `resolveFilterVariable()` and generic `compareValues()` (date → numeric → string ordering)\n- `internal/dataentry/api_v1.go` — new operator cases in `applyV1Filters`; substitute on every filter value\n- Tests: filter handler tests, helper unit tests\n\n## Frontend\n\nNo changes — the existing operator map already maps `<=` → `lte` etc., and the EntityList component already passes config filter values through unchanged.

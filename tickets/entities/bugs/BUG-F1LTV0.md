---
id: BUG-F1LTV0
type: bug
title: Config validator accepts `=~` list-filter operator that no layer can evaluate (Active Tickets / Future Concepts silently empty)
description: |-
    tickets/data-entry.yaml declared `operator: "=~"` (value `ready|in-progress|blocked`) on the active_tickets list and (`exploring|validated`) on future_concepts. `=~` is regex syntax from rela's OTHER filter languages (search `prop:x=~`, calfeed `where`, CLI filter) — the list-filter language never supported it. dataentryconfig's validFilterOperators wrongly allowed `=~` while REJECTING the documented `~`, `in`, and `==`: the validator's set matched neither the docs table (docs/data-entry.md "Static Filters") nor the SPA's OPERATOR_MAP nor the API's wire set. Result: both lists showed zero rows for months, with the filter chip still displaying `status =~ …` as if applied.
priority: high
effort: s
why1: The active_tickets and future_concepts lists were empty because their configured `=~` filter reached the SPA, was silently rewritten to `eq`, and matched the literal string "ready|in-progress|blocked" against single-status values.
why2: The invalid operator shipped because ValidateConfig's validFilterOperators map allowed `=~` — the one gate whose whole job is rejecting an operator the pipeline can't evaluate accepted it at startup.
why3: The validator's set drifted because there was no pin tying it to the documented operator set — it was hand-maintained and diverged in BOTH directions (allowed `=~`, rejected documented `in`/`~`/`==`), and nothing failed when docs, validator, SPA map, and API set disagreed.
why4: Four independent hand-maintained operator sets (docs table, validator map, SPA OPERATOR_MAP, API switch) existed with no cross-layer contract test, so each layer's silent-fallback behavior masked the others' gaps.
why5: 'Systemic: filter-language surfaces in rela (search, calfeed where, CLI filter, list filters) look similar but have different operator sets, and config-time validation was not treated as the contract boundary that must exactly mirror what the runtime evaluates.'
prevention: 'Validator set corrected to the documented set (drop `=~`, add `~`/`in`/`==`) and pinned by TestValidateConfig_DocumentedFilterOperatorsAccepted + TestValidateConfig_InvalidFilterOperator (now using `=~` as the canonical invalid case). Downstream layers no longer degrade silently (see BUG-F1LTP1), so any future drift is loud instead of invisible.'
status: done
---

Fixed on branch `bug/filter-pipeline-and-empty-chips`.

- `internal/dataentryconfig/validate.go`: validFilterOperators = documented set
  (`=`, `==`, `!=`, `~`, `<`, `<=`, `>`, `>=`, `in`); `=~` now fails startup
  validation for both list and kanban filters.
- `tickets/data-entry.yaml`: both `=~ a|b` filters rewritten to `in` with
  comma-separated values. Verified live: active_tickets shows the 22
  ready tickets, future_concepts shows 3.

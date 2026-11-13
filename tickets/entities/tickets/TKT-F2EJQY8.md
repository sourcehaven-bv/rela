---
id: TKT-F2EJQY8
type: ticket
title: Page the data-entry collection reads
kind: enhancement
priority: medium
effort: l
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

TKT-FED6FMM left these `request-path` entries in
`internal/archguard/unpagedread_allowlist_test.go`:

- `internal/dataentry/scopedread.go#scopedHeaders` (3 calls)
- `internal/dataentry/helpers.go#visibleListByTypes`, `#listFromStoreByTypes`, `#listAllFromStore`

`scopedHeaders` is the one ACL-verdict collection read. Its callers (the list Go
path, `executeQuery` for search, dashboard and nav, kanban, export, feed, the
gantt node load) need the whole filtered set in Go for totals, sorting, filters,
conditions and scopes that do not lower. So the fix needs a design decision per
caller, not a change to `scopedHeaders` alone.

Steps:

- Add an id narrowing to `scopedHeaders` for the page scope (`pageIDs`) and the `?q=` free-text hits: `MatchingFaces` on the stamped query, then one header read by ids.
- Push conditions that `queryplan.LowerScope` lowers into the store for `executeQuery`, with `CountMatched` for totals.
- The settings dropdown (`settings_handlers.go`) lists every target entity of every relation type; it needs a search picker in `SettingsView.vue`.
- The command "list" context (`commands.go`) reads the whole list type ungated.
- Convert the `TODO(TKT-FED6FMM)` `assertCallBudget` tests in `internal/dataentry` (querybudget, viewcondition_related, nav_status_budget, dashboard_card, acl_when_related_budget) to `assertBudget`.
- Remove each fixed entry from the allowlist.

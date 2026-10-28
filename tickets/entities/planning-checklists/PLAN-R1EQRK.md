---
id: PLAN-R1EQRK
type: planning-checklist
title: 'Planning: Dashboard cards: list source, server-side values, matrix and ratio'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood. Dashboard cards are config only; the SPA runs each `query:` through `/_search` without a limit and folds counts in the browser from the first page. Cards cannot reuse a list, and there is no matrix or ratio card.
- [x] Scope defined (approved brief G3, ticket 2). In: `list:` source, server-computed values for list cards (count, breakdown, table, matrix, ratio), matrix `tone_by`/`colors`, ratio `match:`/`group_by`, `partial`, frontend tiles, docs. Out (backlog): trend cards TKT-40TEOZ, related-entity axes TKT-G3H9OB, lookups TKT-B4U6EK, date triggers TKT-1LO3KU. `query:` cards keep today's SPA path unchanged (backward compatible); matrix and ratio require `list:`.
- [x] Acceptance criteria:
  1. A card with `list: open_risks` shows exactly the rows the list shows the same principal (type, `filters:`, `condition:`, `query_scope`, ACL row gate, world of the request). Its title links to `/list/open_risks`.
  2. Exactly one of `list:` and `query:`; an unknown list, both, or neither fails config load.
  3. `GET /api/v1/_dashboard/card?name=<dashboard>&card=<index>` returns the computed value of one card. A card hidden by `permission:` returns 404 `card_not_found` like an unknown index (the card list is a UX filter, so this is not a boundary). A `query:` card returns 400 `card_not_computed`.
  4. Rows are row-gated, then redacted once, then folded. A value hidden by `visible:` is left out of a breakdown group, a matrix cell, a ratio group or a table cell, and the response sets `partial: true`. Count and totals still count the row (row existence is already visible through the list).
  5. Read cost is independent of row count (budget test at 10 and 50 rows on storetest.Counting).
  6. Matrix: `rows`, `columns` name enum or integer properties of the list type; enum axes in declared order, integer axes ascending over present values. Each cell `{row, column, count, color}`. With `tone_by` (enum): the cell colour is that of the most severe value present, severity = declared order, last most severe; colour from `colors:` (tone `success warning error info new` or any palette colour from TKT-ZKNWSK), else from the app `styles:` for that type, else none. Rows with a hidden `tone_by` value count but do not colour, and set `partial`. Empty cells are neutral. Without `tone_by` the SPA shades by count. A cell opens `/list/<list>?filter[<rows>]=<r>&filter[<columns>]=<c>`.
  7. Ratio: `match:` names a query scope on the list type; the response gives `matched`, `total` overall and per `group_by` group; the tile shows `matched / total (pct)`.
  8. Load-time checks: matrix axes exist and are enum or integer; `tone_by` is an enum; `colors` keys are `tone_by` values and values are tones or valid colour names; ratio `match` is a declared scope; breakdown/ratio `group_by` exists on the list type.
  9. Tiles are keyboard accessible: matrix cells are links with an accessible name ("<row>, <column>: n").

## Research

- [x] Prior art: `navStatusRows` (nav_status.go) already returns every row of a list for this principal (type, ACL gate, request world, query scope, static filters, condition). Gantt (TKT-MW28U5) pins gate, redact once, then fold. Kanban `column_summary` marks a hidden value "(partial)" (client side).
- [x] Scope matching: `resolvedQueryScope.lower` + `store.MatchingIDs` in one query over the list ids when the scope lowers; otherwise `applyScope` over the headers in one batch.
- [x] Budget harness: `newBudgetAppOn` / `readsFor` / `assertBudget` in querybudget_test.go.

## Approach

- [x] Config (`internal/dataentryconfig`): `DashboardCard` gains `list`, `rows`, `tone_by`, `colors`, `match`, keeping the brief's flat keys. `columns:` already holds a table's column list; on a matrix it is a property name. `DashboardCard.UnmarshalYAML` reads a scalar `columns:` into `MatrixColumns` and a sequence into the table columns, so both read naturally in YAML. On the `/_config` wire the axis is `matrix_columns`. Display modes add `matrix`, `ratio`. Card validation gets cfg and metamodel.
- [x] Server (`internal/dataentry/dashboard_cards.go`): handler `handleV1DashboardCard` on `/api/v1/_dashboard/card`. Resolve dashboard and card by index, re-check `permission:` with `permitsGatedUIElement`, read rows with `navStatusRows`, prime traversals and redact each row once with `visibility.Redact(appRedactor(a))`, then fold per display mode. Pure fold functions over redacted rows (unit-testable). Ratio: scope membership over the raw rows' ids in one batch (the same membership the list view's own scope filter uses). Response type in `internal/apiwire/v1`.
- [x] Colours: `dataentryconfig.ParseColor` for palette colours; tones map to hues (success green, warning orange, error red, info blue, new purple) and the response carries the style class the SPA already paints (`badge-*`, `kit-*`, `custom-*`).
- [x] SPA: `getDashboardCard(name, index)`; `DashboardView.vue` fetches list cards from it and query cards as today; new `DashboardMatrix.vue` and `DashboardRatio.vue` tiles; card link for list cards to the list route.
- [x] Docs: data-entry guide Dashboard section.
- [x] Files: dataentryconfig/config.go, validate.go (+ dashboard card validation file), dataentry/dashboard_cards.go (+tests), api_v1.go route, apiwire/v1/responses.go, frontend api, types, DashboardView.vue, two tile components, tests, GUIDE-data-entry.md.
- [x] Alternatives: (a) values inline in `/_dashboard`, rejected because the schema store prefetches it at app start, so every page load would pay every card; (b) one request per dashboard, rejected because one slow or failing card would block the others, and the SPA already loads cards in parallel.

## Security

- [x] Gate before fold: rows come through the ACL row gate, are redacted once, then folded. Hidden values never enter a group label, a cell, or a colour.
- [x] Membership channel: a `match:` scope or list `condition:` that reads a hidden field decides membership on the raw row, exactly as the list view does; the count discloses no more than the list already does. Documented.
- [x] No cross-principal caching (writeV1JSON is no-store).
- [x] Inputs: `name` resolves against config; `card` must parse as a non-negative int within range; colours are config-validated allowlist names.

## Risk Assessment

- [x] Risk: diverging from the list's own row set. Mitigation: reuse `navStatusRows`; a test compares the card count with the list endpoint's total under ACL.
- [x] Risk: cost. Mitigation: budget test.
- [x] Effort: xl.

## Test Plan

- [x] Config: valid list card; both/neither source; unknown list; matrix axis not enum/integer; tone_by not enum; bad colours key/value; unknown ratio scope; matrix/ratio without list.
- [x] Server unit: fold functions (count, breakdown order and partial, matrix ordering, severity colour, hidden tone_by, integer axis, ratio groups).
- [x] Server integration: handler with ACL policy hiding a field and a row; permission-hidden card 404; query card 400; card count equals list total; budget test 10 vs 50.
- [x] Frontend: DashboardView list card path, matrix tile cell links and labels, ratio tile text.
- [x] Manual: demo project with a matrix and ratio card, screenshots light and dark.

## Design Review

- [x] Design review done (RR-ZUX3FW, RR-NCLPZA, RR-38G262, RR-LQ78QX). Decisions folded in: `/_dashboard` serves each card's config `index`, and the SPA drops a card response whose title or display differs; unset axis values go to `unplaced`; integer axes use declared min..max when span <= 20, else present values capped at 50 with `truncated`; table rows are redacted entity rows, limit default 10 and max 100.

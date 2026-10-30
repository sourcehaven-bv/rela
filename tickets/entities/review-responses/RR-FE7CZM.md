---
id: RR-FE7CZM
type: review-response
title: Refusal call sites for dashboards and view sections are misnamed or guarded
finding: 'Step 2 calls refuseListOnlyColumn from ''the query-card path in dashboard_cards_validate.go'', but no such path walks columns. validateTableCard (dashboard_cards_validate.go:95-112) runs only for list-sourced cards with display: table, and validateDashboardCardSource returns early when meta is nil (dashboard_cards_validate.go:33-35) or the card is query-sourced (:24-28). The view section column loop at validate.go:1908 sits inside the sourceType != "" and GetEntityDef guards (validate.go:1891-1892), so a section whose source does not resolve is skipped. columns_by_type goes through validateLevelColumns (validate_mentions.go:127), so calling the helper from both ''validateLevelColumns and the columns_by_type loop'' reports a busy column twice. validateByTypeFallback (validate_mentions.go:202) walks s.Columns a third time. Fix: call it once per card in the validateDashboardCards loop (validate.go:2607), which visits every card whatever its source and needs no metamodel, and once per section from one walker over Columns, ParentColumns, ChildColumns and ColumnsByType, placed beside validateSectionRender (validate.go:1874) outside the source guards. Add AC 2 rows for a query card and for a section with an unresolvable source.'
severity: significant
resolution: ADDRESSED refuseListOnlyColumn now called once per card in the validateDashboardCards loop and from one section walker over all column lists beside validateSectionRender outside the source guards; validateTableCard, validateLevelColumns and validateByTypeFallback untouched; AC 2 adds query card, non-table card, unresolvable source and a single-report case
status: addressed
---

---
id: RR-RZKL3V4
type: review-response
title: Unfiltered search budget coverage dropped
finding: querybudget_test.go replaced the q=type:ticket budget with a filtered query, so the plain type card shape was no longer pinned.
severity: significant
resolution: 'Addressed: TestQueryBudget_SearchIsSizeIndependent keeps q=type:ticket as a calls-only pin with the by-design reason (no limit returns every match); TestQueryBudget_SearchFilteredIsSizeIndependent and TestQueryBudget_SearchLimitIsSizeIndependent check rows.'
status: addressed
---

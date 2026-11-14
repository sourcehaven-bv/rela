---
id: RR-PQ8VGRO
type: review-response
title: Structured search without a limit was capped at 1000 rows
finding: 'queryservice.go: searchPage capped a structured /_search without a limit at maxStructuredSearchResults. DashboardView computes breakdowns from data and the count from meta.total, so a card over 3000 matches would show a breakdown summing to 1000 with no truncation signal.'
severity: significant
resolution: 'Addressed: the cap is gone. Without a limit the plan reads every match in the store (searchPlan.page with limit 0) and the fallback does not cut, as before the change.'
status: addressed
---

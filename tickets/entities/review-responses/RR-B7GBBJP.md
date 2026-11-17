---
id: RR-B7GBBJP
type: review-response
title: Scope and condition together can exceed the tree limits
finding: listpushdown ANDs a lowered scope and a lowered condition. Each fits the limits, but the combined Where can exceed MaxCondLeaves, and every backend then refuses the query.
severity: minor
resolution: listPlan.narrowed validates the combined Where and declines the pushdown when it is over the limits, so the Go path answers. TestListPlanNarrowed_DeclinesOverLimits pins it.
status: addressed
---

---
id: RR-0U4AV0I
type: review-response
title: Repeated type from the type parameter disagrees between plan and fallback
finding: api_v1.go prepends type:<type> to q, so q=type:task ...&type=task parses as [task, task]. The plan read task once; visibleListByTypes read it twice, duplicating rows and doubling total.
severity: significant
resolution: 'Addressed: visibleListByTypes reads a repeated type once and planSearch dedupes in order. searchPlan_test.go adds the handler-shaped queries, including one the plan declines.'
status: addressed
---

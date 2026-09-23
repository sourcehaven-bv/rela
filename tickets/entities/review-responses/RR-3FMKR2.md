---
id: RR-3FMKR2
type: review-response
title: Validation needs the metamodel and must reuse the list sort rule
finding: validateNavEntry(nav, cfg) has no metamodel (validate.go:535). checkQueryScopeRef returns nil for an unknown type (:2893-2898). The list sort rule accepts id, modified and an empty direction (:80-84, :1093-1106), unlike the plan.
severity: significant
resolution: 'Add a metamodel-aware nav pass: type existence checked explicitly, scope references added to validateQueryScopes as a nav walk, and the list sort rule extracted into a shared helper used by both.'
status: addressed
---

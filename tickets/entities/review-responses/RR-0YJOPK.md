---
id: RR-0YJOPK
type: review-response
title: Field gate silently skipped when GateTraversal gets a nil FieldVisibility
finding: 'GateTraversal takes meta FieldVisibility and accepts nil (skips ConditionallyVisible). The scope resolver is built from (cfg, meta) and has no policy, so the natural wiring passes nil and a conditionally visible far-side property becomes filterable: a binary-searchable value oracle.'
severity: critical
resolution: 'GateTraversal no longer takes a FieldVisibility; it reads the request''s own policy (r.d.policy), so there is no nil to pass. A ConditionallyVisible far-side property is refused per request (ErrTraversalUnsupported) and at boot by QueryScopeTraversalFieldErrors. Tests: TestGateTraversal_RefusesConditionallyVisibleProperty, TestQueryScopeTraversalFieldErrors.'
status: addressed
---

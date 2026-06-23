---
id: RR-FD1A
type: review-response
title: 'Round 1 #1: hidden-stripping not "already done" on cards/list path'
finding: |
  Ticket Scope bullet 3 says "hidden-field stripping is consistently done via stripHiddenProperties, verify it applies." That's wrong. V1ViewEntity is a separate, narrower type from V1Entity. The cards/list converter at api_v1.go:3010 hand-rolls V1ViewEntity and never calls serializeRelatedEntityForWire or stripHiddenProperties. Today the upstream `sec.Fields` view config masks the gap — no per-row ACL hidden-property evaluation runs on this path. Adding `_props` straight from `e.Properties` would introduce a new wire surface that BYPASSES hidden-property stripping. Also: the actual predicate is `App.hiddenProperties(ctx, e) map[string]struct{}` at affordances.go:694, not `IsHiddenForType` as the plan guesses.
severity: critical
status: addressed
resolution: |
  PLAN AC 5 reframed: _props introduces hidden-property stripping to a path that didn't have it. The implementation MUST filter via the existing `App.hiddenProperties(ctx, e)` predicate before populating sed.Props. Ticket Scope bullet 3 rewritten to say "introduce hidden-field stripping for the new _props surface" rather than implying it's already covered. Plan also updates the predicate name from `IsHiddenForType` to `hiddenProperties`.

  New test added to AC 8: seed an entity with a hidden property; assert it's absent from `_props`.
---

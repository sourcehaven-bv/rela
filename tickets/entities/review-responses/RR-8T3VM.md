---
id: RR-8T3VM
type: review-response
title: Consolidate validateCreateIDOpts with EntityDef.MatchesID
finding: EntityDef.MatchesID(id) already does 'does this id match any of my prefixes?' — the new validator re-implements the same loop with slightly different semantics. Could move logic to MatchesIDWithReason on EntityDef.
severity: nit
reason: Suggestion-level. Worth doing but the right home for the merged logic depends on what RR-ODPMN (typed errors) lands on, since MatchesIDWithReason would benefit from the same error-shape. Tracking as a single follow-up that sequences after the typed-errors work.
status: deferred
---

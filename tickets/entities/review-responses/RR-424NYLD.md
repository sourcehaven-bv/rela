---
id: RR-424NYLD
type: review-response
title: Metamodel accepts an alias equal to another type's name
finding: 'Pre-existing and outside this diff: if type a has alias b and type b exists, ResolveAlias may return a depending on map order, so subject: b is refused on some loads. The direction is fail-closed (the flow is refused), and realtimecfg has the same behaviour.'
severity: minor
reason: 'Pre-existing metamodel behaviour outside the scope of #2106. It fails closed, so it cannot widen access; rejecting the collision belongs in schema loading, not flowcfg.'
status: deferred
---

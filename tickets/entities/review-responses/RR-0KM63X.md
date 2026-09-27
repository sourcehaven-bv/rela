---
id: RR-0KM63X
type: review-response
title: Policy-wide field refusal belongs at load
finding: ConditionallyVisible is principal-independent, so a scope filtering on such a property should fail at boot rather than per request.
severity: minor
resolution: 'QueryScopeTraversalFieldErrors runs in prepare after scopes.Compile and fails boot when a traversal filters a property that is not unconditionally visible to every role. Verified manually: rela-server refuses to start with the error naming scope, property and type. Chained incoming hops into relation-granted types are refused per request by gateHop (slot collision); no boot warning added, since it depends on the principal''s grant route.'
status: addressed
---

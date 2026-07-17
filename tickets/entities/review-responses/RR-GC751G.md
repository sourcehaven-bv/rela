---
id: RR-GC751G
type: review-response
title: Tool must look up entity type via store.GetEntity; acl Graph interface can't
finding: who-can needs the entity's TYPE to evaluate roleGrantsRead/grantsVerb (both keyed by type), but ForEntity ignores its type arg (request.go:74) and computeForEntity never knows the type. The acl Graph interface exposes only HasEdge/OutgoingRelations, no entity fetch. So the aclmap engine must take a store.EntityReader dependency and resolve ID->type via store.GetEntity(ctx,id).Type before checking access. This is a required extra store dependency the plan/wiring must include (still a narrow consumer-side interface, not a service locator).
severity: minor
resolution: Plan wires a narrow store.EntityReader into the aclmap engine; entity type resolved via store.GetEntity(ctx,id).Type (same fetch as the existence gate) and used for the type-keyed verb check. Acceptance criterion added.
status: addressed
---

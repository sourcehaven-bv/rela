---
id: RR-T2DQV3D
type: review-response
title: 'Security: gate hops keep the ACL world'
finding: The AllFaces probe must apply to returned rows only; endpoint hops keep the request world.
severity: minor
reason: 'Not an exposure today (TKT-RZSTAPZ). rowProbe sets AllFaces only on the result selection (GraphQuery.Faces). The element range row gate is acl.Request.readQuery: HasInbound and Any branches built by conferredEdge from principal member ids and relation types. Those hops carry no EndpointMatch and are not face-resolved (GraphQuery.Faces godoc), so the world cannot change them. A hop that does read endpoint properties gets InWorld of the request world from GateTraversal, and EndpointPredicate.Faces overrides the probe''s selection. visible: when: conditions never reach the store on this path: hidesSome only decides whether a type folds stored hashes, and every other row is loaded and redacted through the normal read gate. No code change.'
status: wont-fix
---

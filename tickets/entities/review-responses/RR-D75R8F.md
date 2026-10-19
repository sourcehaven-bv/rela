---
id: RR-D75R8F
type: review-response
title: Relation ACL is evaluated with an empty source id
finding: 'internal/entitymanager/linked.go:73-82 prepareRelationFrom authorizes with From = opts.ID, empty for generated ids, so a source-row-scoped relation rule decides differently than CreateEntity then CreateRelation. Fix: authorize the edge inside the Tx once the id is known, or document type-level-only authorization with a test that row-scoped rules cannot be bypassed.'
severity: minor
resolution: 'documented and pinned rather than moved: authorizing inside the Tx would make the pg and sqlite ACL reader miss the uncommitted entity and the relation decision reads only Globals and FromType; TestRelationWrite_DecisionIgnoresSourceID proves an owner role on the source grants no edge and the decision is identical with and without FromID (2ce29a3fe)'
status: addressed
---

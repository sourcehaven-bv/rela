---
id: RR-UM7R2X
type: review-response
title: 'Test gaps: in-Tx type-change reauthorization and real-manager Lua tests'
finding: The DeleteEntityFace branch that re-authorizes when the row's type changed inside the Tx has no test; authorizeCascadeRelations with FamilyFaces had no manager-level test; the Lua tests use a stub manager on a faceless type.
severity: minor
resolution: 'Added manager-level FamilyFaces tests for the cascade (TestDelete_CascadeIdentityEdgeFromFacedSourceNeedsEveryFace) and relation create. The type-change branch is left untested: type is immutable on update, so the branch is reachable only by delete-and-recreate under another type between the early read and the Tx, which no fixture metamodel supports (it needs a second faced type sharing the id). The Lua tests assert the manager boundary; the face semantics behind it are covered by the entitymanager tests.'
status: addressed
---

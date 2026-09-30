---
id: RR-W0H7J6
type: review-response
title: rename_relation_type copies the tail across scopes
finding: A rename between a content-scoped and an identity-scoped type keeps tails the new type does not expect; ShapeProjection records no scope.
severity: minor
reason: Needs relation scope in ShapeProjection which is a projection-format change outside the relation flip. Documented on renameRelationTypeStep. Tracked in TKT-JAD5M9.
status: deferred
---

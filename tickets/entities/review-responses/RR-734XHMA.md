---
id: RR-734XHMA
type: review-response
title: Face on an existing source of an identity relation is silently dropped
finding: relationTail strips the face before requireRelationFaceFor sees it, so the batch accepts what a single CreateRelation refuses with ErrFaceNotDeclared.
severity: minor
resolution: An existing source keeps its raw face; requireRelationFaceFor now refuses a face on an identity relation with ErrFaceNotDeclared. TestCreateBatch_IdentityRelationRefusesTail.
status: addressed
---

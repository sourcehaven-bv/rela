---
id: RR-OP54MI
type: review-response
title: Copy engine and cascade delete authorized relation writes with the face dropped
finding: 'Two ACL subjects were built without the source face while the corresponding write carried it. (1) The copy engine''s per-edge check (copy.go) asked about the default face while applyCopyEdges writes the edge to plan.targetTail. (2) authorizeCascadeRelations deduped by (relType, fromType) only, so one verdict stood for draft- and published-tailed edges alike and a `policy@published` grant covered a draft-tailed edge. Neither was exploitable: both sit behind an entity-level gate that does carry EntitySubject.Face and must already have passed. Both are the check-here/write-there split RelationSubject.FromFace exists to close.'
severity: significant
resolution: 'copy.go passes FromFace: plan.targetTail on the per-edge subject, matching the entity-level check directly above it. authorizeCascadeRelations adds the face to both the dedup key and the RelationSubject; cardinality stays bounded by (types x faces). The FromID comment was updated, since the decision is now a pure function of (relation type, source type, source face, op).'
status: addressed
---

## Finding

Both were raised by the security review as pre-existing but newly fixable —
`RelationSubject.FromFace` did not exist until this change, so neither site
could have been correct before.

**Copy engine** (`internal/entitymanager/copy.go`). `applyCopyEdges` writes
copied edges tailed at `plan.targetTail`, which a cross-entity `to: type@face`
definition makes non-zero. The authorization left `FromFace` zero, so the check
asked about the default face while the write landed on `targetTail`.

**Cascade delete** (`internal/entitymanager/manager.go`).
`authorizeCascadeRelations` deduped by `(relType, fromType)`, collapsing draft-
and published-tailed edges into a single verdict.

## Why neither was exploitable

Both are gated by an entity-level check that does carry the face. A cross-entity
copy never takes the same-entity exemption, so `authorizeCopy` always runs an
`acl.EntitySubject{Face: plan.targetTail}` check first; the cascade sits behind
the entity delete, which carries `EntitySubject.Face`. The gap was that the
*edge* verb was checked one face too loosely, not that the face was unguarded.

Worth fixing anyway: the looseness is only bounded by a neighbouring gate, so
relaxing that gate later would turn it into a real one.

## Resolution

Both subjects now carry the face the write uses. The cascade's dedup key gains
the face too — without that, adding it to the subject alone would still let one
verdict stand for several faces.

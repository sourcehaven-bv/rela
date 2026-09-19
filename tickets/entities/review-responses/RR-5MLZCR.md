---
id: RR-5MLZCR
type: review-response
title: 'Incoming faced relation delete addressed the zero tail: 500 on a legitimate request, link survived'
finding: 'applyRelationsModern derived one tail per relation type from the request address and used it for every write. On the INCOMING path the edge tails at the PEER''s face, which the request does not name, so the delete addressed the zero tail and failed: 500 relation_write_failed, edge intact. Clearing an incoming content-scoped edge from the target side was unconditionally unavailable.'
severity: critical
resolution: An existing edge is now addressed by the tail it carries (tailOf reading Relation.FromFace); only a new edge takes the tail from the request address. Pinned by TestFacedAddress_IncomingEdgeAddressedByItsOwnTail, which fails with the same 500 on pre-fix code.
status: addressed
---

## Finding

`applyRelationsModern` computed one tail per relation type from the request
address and used it for every write. On the INCOMING path that is wrong: the
edge tails at the PEER's face, which this request does not name, so
`currentEdgesByPeerOnFace` deliberately left the set unfiltered while every
write derived from it used the zero tail.

Reproduced: with `POL-1@draft --cites--> FEAT-1` seeded, `PATCH
/api/v1/features/FEAT-1` with `{"relations":{"cited-by":{"data":[]}}}` returned

```
500 relation_write_failed
detail: relation=cites op=delete reason=delete_failed target=POL-1
        cause="delete relation: store: not found"
```

and the edge survived. A legitimate request was unconditionally unavailable, and
the entity PATCH had already committed, so the documented atomicity gap fired on
a routine operation.

The same zero tail reached `writeUpdateRelation` on the incoming update branch.
That one was harmless — `UpdateRelationState` with a mismatched tail returns
`ErrNotFound` rather than writing the default edge, which is the contract the
new conformance case pins.

## Resolution

An edge that already exists is now addressed by the tail it carries (`tailOf`,
reading `Relation.FromFace`); only a NEW edge takes the tail from the request
address. The store's `*State` methods treat the tail as identity, so the address
has to come from the row rather than from the caller — deriving it per call site
is the defect, not the specific arithmetic.

`currentEdgesByPeerOnFace`'s doc now states the obligation explicitly: on the
incoming side the whole set is returned and the caller must address each edge by
its own tail.

Pinned by `TestFacedAddress_IncomingEdgeAddressedByItsOwnTail`, which fails with
exactly the 500 above on the pre-fix code. The fixture gained an explicit
`inverse: { id: cited-by }` on `cites` so the target side is addressable.

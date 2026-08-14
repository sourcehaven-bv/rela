---
id: RR-0V0TVB
type: review-response
title: PatchEntity cannot authorize before reading — needs the entity type for the ACL subject (DeleteEntity shape)
finding: 'The plan implicitly assumes PatchEntity can follow UpdateEntity''s authorize-first shape. It cannot. acl.WriteRequest.Subject is acl.EntitySubject{Type, ID} (manager.go:561-563); UpdateEntity gets Type free because the caller hands it a whole *entity.Entity, but PatchEntity(ctx, id string, p EntityPatch) has only an ID. It must read first to learn the type. This is exactly DeleteEntity''s shape (manager.go:663-673), which documents the accepted tradeoff: ''ACL check happens after the lookup so the request carries the real entity type; a deny on a non-existent entity would be more confusing than the ErrEntityNotFound returned above.'' So PatchEntity inherits DeleteEntity''s already-accepted existence disclosure — defensible for the same reason, but it must be a stated, deliberate inheritance rather than an accident, and it constrains where the field gate can sit (strictly between authorize and mutation).'
severity: significant
resolution: |-
    ACCEPTED. PatchEntity adopts the DeleteEntity shape (manager.go:663-673): raw GetEntity first to learn the real entity type, then authorize with acl.EntitySubject{Type: stored.Type, ID: id}. Documented in PatchEntity's godoc citing DeleteEntity's existing rationale, so the existence disclosure is an explicit, precedented inheritance rather than an accident.

    Rejected alternative (recorded so it is not revisited): taking `type` as a PatchEntity parameter to enable authorize-before-read. That would let a caller assert a type disagreeing with the stored row — the exact class ApplyEntity guards with ErrTypeImmutable (apply.go:105-110). Reading the real type is safer.

    This finding is what fixes the position of the field gate in RR-32XA5V: the gate sits strictly between authorize and mutation, since authorize cannot move earlier.
status: addressed
---

## Evidence

`internal/entitymanager/manager.go:663-673` (`DeleteEntity`):

```go
current, err := m.deps.Store.GetEntity(ctx, id)
if err != nil {
    return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
}
// ACL check happens after the lookup so the request carries the
// real entity type; a deny on a non-existent entity would be more
// confusing than the ErrEntityNotFound returned above.
if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
    Op:      acl.OpDelete,
    Subject: acl.EntitySubject{Type: current.Type, ID: id},
}); aclErr != nil {
```

## Resolution

Adopt the `DeleteEntity` ordering explicitly and document it in `PatchEntity`'s
godoc, citing the same rationale. The existence disclosure is pre-existing and
accepted for ID-addressed write ops; what must NOT happen is the additional
field-policy disclosure from RR-32XA5V, which is why the gate sits after
authorize rather than before it.

An alternative — taking `type` as a `PatchEntity` parameter so it could
authorize first — is rejected: it would let a caller assert a type that
disagrees with the stored row, which is the exact class `ApplyEntity` guards
with `ErrTypeImmutable` (`apply.go:105-110`). Reading the real type is safer.

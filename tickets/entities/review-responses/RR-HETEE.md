---
id: RR-HETEE
type: review-response
title: 'Create-path: EnforceCreate runs after persist -> orphan row indexed, SSE-broadcast, un-audited on rejection'
finding: 'manager.go:436 runs EnforceCreate AFTER createCore persisted the row (manager.go:419 -> Store.CreateEntity) and BEFORE recordEntityAudit (manager.go:448). fsstore.CreateEntity emits+notifies observers synchronously, so on an ErrIllegalEntry rejection the entity is already on disk, indexed by search, broadcast to SSE clients as entity-created, and has NO audit row. Caller gets 422 believing the create failed; the system half-committed it with no forensic trail. The update path correctly checks before Store.UpdateEntity; create has the check on the wrong side of the write. Fix: run EnforceCreate on the candidate entity BEFORE the store write (the resolved property values exist pre-persist inside createCore); entry-value check needs no persisted row.'
severity: critical
resolution: EnforceCreate moved into createCore BEFORE Store.CreateEntity (core.go), so an illegal entry is rejected without ever persisting a row or emitting a store event (no orphaned index entry, no SSE broadcast, no un-audited row). Removed the post-persist check in manager.go CreateEntity. Regression test TestTransition_IllegalEntry_DoesNotPersist asserts zero rows exist after a rejected illegal-entry create.
status: addressed
---

## Finding

`CreateEntity` runs `EnforceCreate` (manager.go:436) **after** `createCore`
already persisted the row (manager.go:419 → `Store.CreateEntity`) and **before**
`recordEntityAudit` (manager.go:448).

`fsstore.CreateEntity` emits + notifies observers synchronously
(`fsstore/entity.go:227`, `fsstore.go:334-336`). So on an `ErrIllegalEntry`
rejection, before the error returns:
- the search index has indexed the entity,
- the SSE bridge has broadcast an entity-created event to every data-entry client,
- and the audit log has **no** record (audit is at 448, after the failed check).

## Concrete failure

POST create with `status=established` on a machine whose entry is `in-review`.
Server returns 422. Meanwhile the entity is on disk, an SSE client rendered a
card for it, search returns it, and there is no audit row. The caller believes
the create failed; the system half-committed it, silently.

## Resolution

Run the entry-value check on the candidate **before** the store write — the
resolved post-template property values exist pre-persist inside `createCore`
(before core.go:110). Mirrors the update path, which correctly checks before
`Store.UpdateEntity`.

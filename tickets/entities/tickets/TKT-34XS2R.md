---
id: TKT-34XS2R
type: ticket
title: 'Optimistic concurrency at the store: expected-version on entity.Patch / UpdateEntity'
kind: enhancement
priority: high
effort: l
tags:
    - needs-design
status: in-progress
---

rela has **no compare-and-swap for updates**. `store.Store.UpdateEntity(ctx, e)`
takes an entity and writes it; `entity.Patch` carries `Properties`, `MetaUnset`
and `Content` and nothing that says "only if unchanged". Every read-modify-write
in the codebase is therefore protected by `writeMu` — one process-wide mutex —
or by nothing.

## Why this matters more than it looks

**The data-entry API's `If-Match` is not a CAS.** `write_handler.go` reads the
stored entity, computes `computeEntityETag`, compares it to the header, and then
writes — all inside `writeMu` (`enterWrite` at :346, compare at :376). It is a
check-then-write that is safe *only* because a process-local lock serializes it.
That is precisely the shape `Manager.CreateEntity` deliberately removed:

> No GetEntity pre-check: it was a TOCTOU duplicate of the store's atomic
> uniqueness guarantee.

Creates got a real atomic guarantee (the postgres derived unique index). Updates
never did.

**`docs/postgres-backend.md` documents several rela-server processes against one
database.** `writeMu` spans one process, so between processes there is no
serialization and no conflict detection on the update path at all. This is
already demonstrated in-tree: `TestWebhookConflict_CrossProcessAppendsCanBeLost`
asserts **1 of 2 appends lost** across two stores on one schema.

**Removing the lock is not an option today.** Verified by experiment: deleting
`writeMu` from the webhook pipeline makes
`TestWebhookConflict_PipelineAppendsAllLand` fail on every run, because
`Patch.Content` is an absolute replacement computed from a base read moments
earlier.

## Scope

- **`entity.Patch` gains an expected-version** (or expected-content-hash) field,
and `store.Store.UpdateEntity` gains the contract: the write applies only if the
stored record still matches, else it returns a conflict error the caller can
distinguish and retry on.
- **Every backend implements it** — fsstore, memstore, sqlitestore, pgstore —
and it becomes part of `internal/store/storetest` so a new backend cannot skip
it. Postgres and sqlite get a `WHERE version = $n` predicate; the file/memory
tiers need their own equivalent.
- **Callers migrate**: the data-entry PATCH path stops relying on `writeMu` for
correctness, and the webhook `append_section` step (TKT-1EM4KL) drops its lock.

## What this fixes

1. The webhook append clobber, cross-process — the case `unique:` cannot help
with, since nothing is created.
2. The data-entry API's `If-Match` TOCTOU, which is currently lock-dependent.
3. Any future caller doing read-modify-write, which today has no primitive to
use and will reach for a lock instead.

## Design questions

- **Version token shape.** A monotonic per-entity counter, or a content hash?
A hash needs no schema change on fs/mem and matches the existing
`computeEntityETag` (sha256 over id, type, content, sorted properties, outgoing
relations); a counter is cheaper to compare and easier to index, but is new
state every backend must carry and keep correct.
- **Whether the API's ETag becomes the token**, so `If-Match` maps straight
through instead of being recomputed and compared above the store. Attractive,
but the ETag folds in outgoing relations, which a store-level compare may not
want to be sensitive to.
- **Relations.** `UpdateRelation` has the same gap; whether it is in scope or a
follow-up.
- **Interaction with `store.Store.Tx`** (DEC-8UIL0). A CAS inside a transaction
is redundant on postgres and load-bearing outside one; the contract must say
which.
- **Retry ergonomics.** Callers need a distinguishable conflict error, in the
shape `errors.Is`/`errors.As` can match — note the near-miss in [[RR-HI9QIU]],
where a translated error erased the cause and silently disabled a retry loop.

## Out of scope

- Multi-entity transactions. This is per-record CAS, not a unit of work.
- Removing `writeMu` wholesale. It also serializes non-entity writes; this
ticket removes the *correctness* dependency on it for entity updates, and the
DEC-8UIL0 `Tx` arc owns the rest.

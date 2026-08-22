---
id: TKT-8HDPQW
type: ticket
title: 'ACL increment 3: cascade delete authorizes incident relations under Store.Tx (B1)'
kind: enhancement
status: in-progress
priority: high
effort: m
---

## Increment 3 of 3

Depends on **TKT-K2VN9D** (the `relation_grants:` block gives the per-relation
`delete:` permission this enforces). Split from **TKT-XZEY**; the design was
materially corrected at design review (DR-1).

## The gap

`Manager.DeleteEntity` (`manager.go:847`) makes exactly **one** ACL call — the
ENTITY delete at `:855-861`. Incident relations are collected at `:862-869` for
versioning, then the whole deletion is delegated to
`Store.DeleteEntity(ctx, id, cascade)` at `:911`, which bulk-deletes relations
below the write choke-point.

So a principal who may delete entity type X destroys **every** incident
relation of **any** type, including types they hold no relation-delete grant
on. Deleting an entity must not be a back door to destroying edge types you
cannot delete directly.

## DR-1 — the naive fix does NOT work

The obvious implementation ("authorize the already-collected `incoming`/
`outgoing` before the store call") was the original plan and it is **wrong**.
It authorizes a snapshot collected **outside any lock**
(`ListRelations`, `core.go:240-254`), and both stores then **independently
re-derive** the relation set inside their own lock/tx and delete *that* set:

- fsstore rebuilds from the live index under `s.mu.Lock()`
  (`fsstore/entity.go:308-313`) — verified.
- pgstore re-reads inside the tx (`pgstore/entity.go:371-376`) and deletes with
  an unqualified `DELETE FROM relations WHERE from_id=$1 OR to_id=$1` (`:382`);
  the tx begins at `:357`, **after** the manager's collection.

The window `[collect :869, store lock :911]` admits new incident relations from
any concurrent writer, each deleted with **zero authorization**.

Postgres transactionality does **not** save it: `READ COMMITTED` sees rows
committed after the tx began, and **the transaction does not span the
authorization**. "pg is transactional anyway" is a non-sequitur here.

**Attack:** a low-privilege principal races
`CreateRelation(victim, sensitive-rel, X)` against a cascade
`DeleteEntity(victim)` by a principal lacking delete on `sensitive-rel`. The
pre-flight never sees the edge; the store deletes it.

## Approach

Run **collect + authorize + delete** inside `store.Store.Tx` — the sanctioned
transaction seam (DEC-8UIL0), giving mutual exclusion on fs/mem and snapshot
isolation on pg.

**Verify first (blocking spike):** the ACL evaluator does graph reads
(`resolver.go:220`, `r.d.graph.OutgoingRelations`). Confirm they do **not**
route through the outer store handle, or fsstore self-deadlocks —
`fsstore/tx.go:22-27` warns about exactly this. If they do, resolve before
implementing; do not discover it mid-PR.

**Fallback if the spike says no:** have the store return its deleted set and
re-verify `res.DeletedRelations` against the authorized set **after** the fact,
failing loudly and audit-flagging any difference. Strictly weaker (the deletion
already happened) but it converts a silent hole into a detectable one. Record
which option shipped and why.

**Dedupe by `(relationType, op)` before authorizing (DR-21).** The decision is a
pure function of relation type + source type + op, and cascade delete has one
op. A hub entity with 5,000 edges across 6 relation types becomes 6 checks
rather than 5,000 — and, on denial, one audit row per distinct relation type
instead of 5,000 for a single refused operation.

## Behaviour change to document

An entity delete can now fail on a **relation** grant. Intended least-privilege
semantics, but operators must be told. **Wrap the error (DR-16/F3):** a raw
"no role grants delete on relations from type X" is confusing when the operator
asked to delete an entity — say *"cannot delete <id>: its <relType> relation to
<peer> requires ..."*.

## Acceptance criteria

1. Principal may delete entity type X, holds no delete grant on relation type
   R, entity has an incident R edge → `DeleteEntity(cascade=true)` DENIED **and
   the entity still exists**.
2. **Assert (1) on memstore, not only postgres (DR-20).** The postgres variant
   needs `RELA_TEST_DATABASE_URL` and is skipped in default CI; if that is the
   only place the property is asserted, a regression ships on the default build.
3. **Race test:** a relation created concurrently between collect and delete is
   either authorized or the delete fails — never deleted unauthorized. This is
   the test that distinguishes the correct fix from the naive one.
4. Denial produces one audit row per distinct relation type, not per edge.
5. Error message is entity-delete-shaped, naming the blocking relation.
6. `cascade=false` behaviour unchanged (`ErrHasRelations`).
7. A principal holding delete on both the entity type and every incident
   relation type succeeds, with relation versions still captured
   (`manager.go:894-902`).

## Out of scope

- Automation relation writes → TKT-M3W8PK.
- Entity rename re-key (B2): relation SET is unchanged, only endpoint ids move,
  covered by the entity `rename` grant. **Deliberate decision, not an
  oversight.** No caller pairs `OpRename` with a `RelationSubject`, so
  `grantsVerb`'s `OpRename→Update` routing is unreachable for relations —
  TKT-JW03LN pins that with a test.

## Files

`internal/entitymanager/manager.go`, `internal/entitymanager/manager_test.go`,
`docs/acl-security.md`.

---

## Implementation notes (2026-08-22)

### The blocking spike PASSED

The design depended on "does the ACL evaluator's graph read re-enter the store
and deadlock under `Tx`?" Answer: **no**.

- `fsstore.go:134-138` states it outright — *"Readers never take it [txMu]"*.
  `txMu` serializes an open Tx against **writers** only.
- `acl.StoreGraph` (`storegraph.go:38,53`) only ever calls `GetRelation` and
  `ListRelations` — pure reads.
- Pinned empirically by a new **`TestTx_ReadsViaOuterHandleDoNotDeadlock`**
  (`internal/store/fsstore/txreentrancy_internal_test.go`), which reads via the
  OUTER handle inside a Tx and writes via the view. The package documented this
  property but nothing asserted it; a future change giving readers `txMu` would
  have hung the server rather than failed an obvious test.

### What landed

`DeleteEntity` now runs collect → authorize → delete inside one
`m.deps.Store.Tx`. `authorizeCascadeRelations` checks every incident relation
and returns the first denial; nothing is written before the check, and a denial
fails the whole delete.

**Dedup by `(relType, fromType)`** (DR-21): the decision is a pure function of
those plus the op, so a 5,000-edge hub costs a handful of checks — and a denial
writes one audit row per distinct relation type instead of 5,000 for one
refused operation.

**Directional error text.** Each relation is authorized against ITS OWN source
type, which is the same subject the principal would face deleting that edge
directly. For an *incoming* edge the deleted entity is the `To` side, so the
message says "its incoming X relation from Y" rather than naming the wrong end.

### A real seam question the tests exposed

Two existing test fakes (`failingDeleteStore`, `listRelationsErrStore`) embed
`store.Store` and override one method. Moving the delete under `Tx` broke both:
the embedded `Tx` hands the callback the **inner** store, so the override became
invisible inside a transaction and the fakes silently stopped modeling the
failures they exist to model.

Fixed by giving each fake a `Tx` that passes ITSELF to the callback. Worth
recording as a general property: **any decorator over `store.Store` must
override `Tx` or it is bypassed inside one.** Production is unaffected — the
store handed to entitymanager is the concrete backend, with no decorators — but
this is a trap for any future wrapping store.

### Backend semantics confirmed

- **pgstore**: `Tx` takes `pg_advisory_xact_lock` for the whole transaction
  (`tx.go:60-93`), serializing write transactions deployment-wide, and
  `tx.DeleteEntity` joins the open transaction. So no concurrent writer can add
  an edge between the authorize and the delete.
- **fsstore/memstore**: `txMu` gives mutual exclusion for the same window.

Note the ACL's reads still go through the outer handle (that is what
`acl.StoreGraph` holds, bound at construction). On pgstore that is a different
connection, so those reads see a pre-transaction snapshot — but the advisory
lock means **no writer can change the relation set during the callback**, so the
window is closed by the lock rather than by read isolation. Re-pointing the ACL
at a tx view per call is not possible without threading a store through
`acl.Request`, and is not needed for this guarantee.

### Note on mutation testing this gate

An automated security scanner flagged `if false { ... authorizeCascadeRelations }`
as a CRITICAL fail-open authorization bypass mid-session. **It was correct to
flag it** — that was a deliberate, temporary mutation to prove the new tests
detect a disabled gate, and it should never reach a commit.

Two process lessons, both worth keeping:

1. **Mutate a COPY, never the working tree.** In-tree mutate/restore is unsafe
   when the test run can be interrupted (it was, twice, by unrelated `go test`
   contention) — the tree is then left with authorization disabled. Subsequent
   runs used `cp -R` to `/tmp` so the real tree is never modified.
2. A scanner watching the working tree is a genuine safety net for exactly this
   mistake, and the right response is to restore immediately and verify, not to
   explain the intent and move on.

The committed code has the gate unconditional; `grep -c 'if false'` in
`manager.go` is 0.

### Refactor forced by the linter

Extracting the Tx body into `deleteEntityInTx` was not cosmetic: inlining the
critical section pushed `DeleteEntity` to cognitive complexity 32 (limit 30).
The named method also gives the load-bearing sequencing — collect, authorize,
version, delete, all under one transaction — a place to be read on its own.

---
id: TKT-GXHI8
type: ticket
title: Add Tx write-transaction contract to store.Store with fs/mem/pg implementations (DEC-8UIL0 phase 1)
kind: enhancement
priority: medium
effort: m
status: done
---

First slice of the [[DEC-8UIL0]] store-transaction arc (researched in
[[RES-Z1SJ5]]): add `Tx(ctx, fn func(Store) error)` to the `store.Store`
interface and implement it in all three backends. No callers change in this PR —
entitymanager wiring, the Lua `rela.tx` helper, and writeMu deletion are later
slices.

## Scope

- `store.Transactor` interface embedded in `store.Store` (interface member, not
an optional capability — every backend implements it).
- fsstore/memstore: `txMu` serialization — public write methods acquire it
briefly; `Tx` holds it across the callback and hands a view that skips it
(re-entrancy by structure; Go has no reentrant mutex). **Reduced guarantees
accepted for single-user fs deployments:** no rollback (an error mid-callback
leaves earlier writes applied), events emitted inline per write, watcher
reconciliation of external file edits not excluded.
- pgstore: `Tx` = `BEGIN` + global `pg_advisory_xact_lock` (new write key,
distinct from sweep/migrate keys) + a tx-bound store view; per-write methods run
as savepoints via the existing `DBTX` seam; `pg_notify` defers to outer commit
natively; in-process observer/subscriber notifications buffered and replayed
after commit; rollback on error.
- Nested `Tx` joins (no nested-transaction API).
- storetest: shared `RunTxTests` in `RunAll` (visibility, read-your-writes,
serialized read-modify-write under concurrency, error propagation, nested join)
plus `RunTxRollbackTests` opted into by pgstore only.
- Root CLAUDE.md rule amendment per the decision.

Out of scope (later slices): entitymanager intent wrapping, Lua `rela.tx`,
writeMu deletion, fs rollback journal, per-entity lock granularity.

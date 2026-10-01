---
id: BUG-9TGOH1
type: bug
title: pgstore iterators hold a pool connection across yield, deadlocking the pool under concurrent listings
description: pgstore list/query iterators keep rows (and their pool connection) open while yielding; caller code that makes a nested store call per row (visibility redaction) needs a second connection, so N concurrent listings on an N-connection pool deadlock the whole server. neoq's JobTimeout does not cancel the handler ctx, so it never recovers without a restart.
priority: critical
effort: m
why1: 'pgstore''s ListEntities, ListEntityHeaders, GraphQuery, GraphQueryHeaders, ListRelations and SearchVisible call yield inside their rows.Next() loop, and SearchVisibleFields calls the caller''s HiddenFieldsFunc there. The pgx connection stays checked out while caller code runs. visibility redaction (FieldVerdicts -> acl HasEdge -> GetRelation) needs a second connection per row, so N concurrent listers on an N-connection pool each hold one and wait for another. Reproduced: all seven fail with context deadline exceeded at MaxConns=2 with four listers.'
why2: 'The iterators were written as thin wrappers over a pgx cursor (TKT-M8400, #893), which is the natural shape of iter.Seq2 over a database. Streaming keeps memory flat, but it silently extends the connection''s lifetime to the caller''s loop body, which the store does not control.'
why3: store.EntityReader documents iterator semantics (termination on error, ignored Cursor/Limit) but not resource semantics. Nothing says whether a caller may make store calls inside the loop. fsstore and memstore iterate in-memory data, so nesting is free there, and every consumer (visibility.redactingSeq, Lua list_entities, dataentry includes) was written against that assumption. The only acknowledgment is a comment in ListRelations that closes rows before its OWN follow-up query on a Tx view.
why4: The conformance suite runs pgstore on a 2-connection pool but never nests store calls inside an iteration under concurrency. TestTxPoolExhaustion pins the 'no second connection while holding one' property for Tx only. Unit and e2e tests run few concurrent requests, so the pool was never saturated; production needed four scheduled tasks starting in the same second.
why5: 'Connection ownership is not part of the store contract or its conformance harness. A backend that holds an external, bounded resource across a callback can pass every test while violating an invariant only the deployment exposes. Recovery was also absent: neoq''s handler.Exec passes the parent ctx (not its timeoutCtx) to Handle, so JobTimeout frees the worker slot but never cancels the stuck goroutine, and pool waits have no metrics or slow-acquire warnings that would have surfaced the contention (it first showed only as context-canceled warnings where ACL errors become no-edge).'
prevention: 'The store contract now states that an iterator must not hold a backend resource across yield (store.EntityReader, GraphQueryer, VisibleSearcher docs). storetest.RunIteratorNestingTests runs in RunAll for every backend: concurrent iterations outnumbering the pool, a nested store call per row, and the same inside a Tx. pgstore runs its conformance suites at page size 2 so multi-page paths are always exercised. neoq dispatch now enforces the handler deadline itself.'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

## Summary

pgstore's streaming iterators yield while their query's rows are still open, so
the pool connection stays checked out for the duration of the caller's loop
body. Visibility redaction makes a nested store call per row (ACL `HasEdge` ->
`GetRelation`), which needs a second connection. N concurrent listings on a pool
of N connections deadlock.

Same "no second connection while holding one" property `TestTxPoolExhaustion`
pins for `Tx`, broken by the iterators.

## Impact (production, rela-server-postgres, default pool of 4)

- Four scheduled Lua tasks calling `list_entities` deadlocked at once.
- Every HTTP request then blocked in `pgxpool.Acquire` (principal resolution); UI showed no data. Reaper, LISTEN catch-up, sweep and migration GC also stuck.
- neoq's job timeout did not recover it; only a restart did.
- Precursor: bursts of `context canceled` warnings where an ACL backend error is treated as "no edge", so users silently see less data.

## Affected methods

`ListEntities`, `ListEntityHeaders` (entity.go), `GraphQuery`,
`GraphQueryHeaders` (graphquery.go), `ListRelations` (relation.go),
`SearchVisible`, `SearchVisibleFields` (visiblesearch.go).

## Secondary defect

`jobs.neoqQueue` relies on neoq's `JobTimeout`, which in v0.72.1 does not cancel
the handler's ctx. A wedged handler keeps its connection forever.

Source report: `~/Downloads/rela-pgstore-iterator-pool-deadlock.md`.

## Reproduction

`internal/store/pgstore/iterator_pool_test.go`
(`TestIteratorsDoNotHoldConnectionAcrossYield`): scoped pool with `MaxConns=2`,
four concurrent listers, each making a nested `GetRelation` per row (for
`SearchVisibleFields`, also inside the `HiddenFieldsFunc`). All seven subtests
fail with `context deadline exceeded` on develop (de764598), local PostgreSQL.

```bash
RELA_TEST_DATABASE_URL="postgres://jeroen@127.0.0.1:5432/rela_test?sslmode=disable" \
  go test -tags postgres -run TestIteratorsDoNotHoldConnectionAcrossYield ./internal/store/pgstore/
```

The neoq claim is confirmed in the fork in use (`sourcehaven-bv/neoq`
c4c1564854aa): `handler.Exec` builds `timeoutCtx` but calls
`handler.Handle(ctx)` with the parent context.

## Fix plan

1. **pgstore iterators read a page fully and close rows before yielding.**
   - `ListEntities`, `ListRelations`: loop over `ListEntitiesPage` / `ListRelationsPage` with an internal page size (keyset on id, no `OFFSET`). `ListRelations` keeps its reveal pass after the last page.
   - `ListEntityHeaders`: add a header page helper over `buildEntityHeaderListSQL` with the keyset argument.
   - `GraphQuery`, `GraphQueryHeaders`: when the caller sets `Limit`, materialize (bounded). Otherwise page with a keyset on the full order tuple (`OrderBy` keys, then id).
   - `SearchVisible`: materialize the hits (id/type/title only).
   - `SearchVisibleFields`: buffer scanned rows, close rows, then call `hidden`. Keep the memory bound by scanning in chunks.
2. **Contract.** Document on `store.EntityReader` / `RelationReader` / `search.VisibleSearcher` that an iterator must not hold a backend connection or cursor across yield, so callers may make nested store calls in the loop body.
3. **Conformance test** (`store-iterator-nested-call-conformance`): the reproduction moved into `storetest`, plus a nested-call-inside-`Tx` case, run by every backend.
4. **Recovery.** `neoqQueue.dispatch` wraps its ctx in `context.WithTimeout(ctx, handlerTimeout-handlerCancelGrace)` with a test that the handler sees the deadline. Report upstream (acaloiaro/neoq).

Trade-off: a paged iteration is no longer a single snapshot. A row inserted or
deleted between pages may be missed or seen, depending on whether its id sorts
before or after the cursor. With a keyset, no row is seen twice. A `Tx` does
not restore the snapshot: pgstore's `Tx` runs at READ COMMITTED and takes the
write lock. Keys are typed and never parsed back, so a legacy id at a page
boundary cannot restart the listing.

## Related areas

- `sqlitestore` iterators also yield with rows open. `database/sql` has no `MaxOpenConns` set there, so no pool deadlock, but the conformance test's `Tx` case will show whether nesting inside a transaction works.
- Out of scope, follow-up: pgx pool metrics and a slow-acquire warning. Raising `MaxConns` is a mitigation only.

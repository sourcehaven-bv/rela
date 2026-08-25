---
id: TKT-G91TBK
type: ticket
title: 'SQLite store backend: conformance-passing minimal store behind a sqlite build tag'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Build the SQLite `store.Store` backend decided in DEC-LFSYNY: single-process,
`modernc.org/sqlite`, at pgstore's transaction tier. This ticket is **stage 3**
— the conformance-passing store. Versioning and `StateKV`/`UserState` (stage 4)
are a separate ticket; FTS5 search, SQL pushdown and `DerivedSchemaReconciler`
are deliberately later still.

Depends on TKT-415WA7 (widening the `*pgstore.Store` assertions) only for the
stage-4 capabilities, not for this ticket.

## Carry forward from the spike — do not rediscover

The spike (`spike/sqlite-tx-TKT-TWIO11`,
`internal/store/sqlitespike/RESULTS.md`) established these by measurement. They
are cheap to get wrong and expensive to debug:

1. **PRAGMAs go in the DSN (`?_pragma=...`), never `db.Exec`.** A PRAGMA is
per-connection; `db.Exec` configures one pooled connection and leaves the rest
at the default, while reading back correctly. Measured: `busy_timeout` set via
`db.Exec` fails at 0.00s instead of waiting 5s. Port `verifyBusyTimeout` — it
pins two connections open simultaneously and asserts both agree, so the
misconfiguration cannot return silently.
2. **`BEGIN IMMEDIATE`, always.** Deferred transactions fail the stress test:
a read-then-write must upgrade its lock mid-flight, and the upgrade cannot wait,
so `SQLITE_BUSY` is returned regardless of `busy_timeout`.
3. **Never serialize by shrinking the pool.** `MaxOpenConns(1)` deadlocks —
`Tx` pins the only connection while readers block for one. That is
`database/sql` pool starvation, not a SQLite lock. Use a normal pool plus an
in-process write mutex.
4. **Nested `Tx` must return the same view without acquiring a connection**
(`pgstore/tx.go:61-63` shape).
5. **`storeutil` validators are the oracle.** The spike's `CreateRelation`
initially accepted an empty relation type; `FuzzRelationKeyCollision` caught it
on the seed corpus. Validate IDs and relation types on every write path.

## Scope

IN:

- The 26 `store.Store` methods. `GraphQuery`/`GraphCount`/`MatchingIDs`
delegate to `graphquerynaive` (fsstore's adoption is 28 lines).
- `storetest.RunAll` + `RunTxStressTest` + `RunTxRollbackTests` + all six fuzz
targets. Take the strong Tx contract — the spike showed SQLite gives rollback
and post-commit events for free.
- `HeaderReader` (cheap, pure win).
- Single-writer advisory lock on a **sidecar** file (never the DB file:
SQLite holds POSIX locks on that inode and any fd close drops them). PID +
hostname written before locking; holder identity treated as advisory.
- Refuse to start, or warn loudly, when `journal_mode` is not `wal` — this is
the network-filesystem guard, and it costs nothing since `Open` already reads
the value back.
- Pin `modernc.org/libc` to the version from `modernc.org/sqlite`'s `go.mod`
(upstream #177), with a `go.mod` comment saying why.

**Split out to TKT-L1A3PH** during implementation: the `sqlite` build tag,
`appbuild_sqlite.go`, `cli/db_sqlite.go`, GoReleaser entries, CI isolation
assertions and `justfile` build-check-tags. The store and the wiring are
independently reviewable, and the store is provable on its own — so they are two
stacked PRs rather than one large diff touching store, appbuild, cli, release
config and CI at once.

OUT: versioning, `StateKV`, `UserState`, FTS5 native search (use bleve initially
— `search.Visible` wraps any `Searcher`), SQL pushdown,
`DerivedSchemaReconciler`, `ManifestSince`, multi-process anything.

## Acceptance criteria

1. `storetest.RunAll` passes with `Capabilities{Attachments: true}`.
2. `RunTxStressTest` passes at `RELA_STRESS_SECONDS=30` with no watchdog
deadlock dump, no lost updates, pair atomicity holding.
3. `RunTxRollbackTests` passes — SQLite is wired at pgstore's tier.
4. All six fuzz targets pass.
5. A second process opening the same database fails with a clear error naming
the holder; it never corrupts silently.
6. Startup refuses or warns when WAL is not in effect.
7. ~~CI asserts the default build does not link the sqlite driver, and the
sqlite build links neither pgx nor bleve inappropriately.~~ **Moved to
TKT-L1A3PH** — there is no sqlite binary to assert about until the build tag
exists. The half that IS verifiable here holds: neither the default nor the
postgres build links `modernc.org/sqlite`.
8. `just arch-lint`, `just lint`, `just coverage-check` pass.

## Not licensed by this ticket

Multi-process SQLite; replacing pgstore for shared servers; making SQLite the
`rela-desktop` default (a separate call once the backend exists and has been
exercised).

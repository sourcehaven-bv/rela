---
id: DEC-LFSYNY
type: decision
title: Proceed with a SQLite store backend for single-process desktop, at pgstore's transaction tier
context: 'rela had a large gap between fsstore (markdown files, full-graph residency, git for history) and pgstore (indexed, versioned, but needs a database server). A single-server or desktop deployment wants pgstore''s properties at fsstore''s operational cost. TKT-TWIO11 settled the driver (modernc.org/sqlite, pure Go, no CGO); RES-03TUXO recommended targeting single-process desktop; a throwaway spike then measured the Transactor semantics that could not be established from documentation, because the two relevant upstream issues (#192, #232) are open with no conclusive resolution.'
consequences: 'GO. SQLite lands at pgstore''s transaction tier, not fsstore''s: real rollback and post-commit-only events, giving up only cross-process serialization. Accepted trade-offs: single-process only (enforced by an advisory lock, not documented and hoped for); no git-diffable markdown on this backend, with content versioning as the replacement per the precedent in docs/postgres-backend.md; synchronous=NORMAL means a crash can lose recently committed transactions; no custom FTS5 tokenizers ever; network/sync filesystems unsupported (unmeasured — no network storage available). Prerequisite: three *pgstore.Store concrete-type assertions in appbuild must be widened to interfaces before a third smart backend can opt into versioning, state or derived-schema.'
date: "2026-08-23"
status: accepted
---

## Decision

Build a SQLite `store.Store` backend for **single-process desktop and
single-server** deployments, using `modernc.org/sqlite`, at **pgstore's
transaction tier**. Staged, with the concurrency gate now passed.

## Evidence

Measured on the throwaway spike (`spike/sqlite-tx-TKT-TWIO11`,
`internal/store/sqlitespike/RESULTS.md`). Numbers, not adjectives.

### The finding that mattered most

**`busy_timeout` must be set via the DSN `_pragma=` parameter, not
`db.Exec("PRAGMA ...")`.** A PRAGMA is per-connection: `db.Exec` configures
whichever pooled connection serves that call and leaves every later connection
at 0, while reading back correctly.

| Configuration | Second writer (holder holds a 2s write tx) |
|---|---|
| `db.Exec("PRAGMA busy_timeout=5000")` | fails at **0.00s**, `SQLITE_BUSY` |
| DSN `?_pragma=busy_timeout(5000)` | waits **2.05s**, succeeds |

This reproduces upstream #192 exactly and plausibly explains #232 — mattn takes
`_busy_timeout` in the DSN, so a straight port that moves it into `db.Exec`
silently loses it. **Neither issue reproduced as a driver defect once the DSN
form was used.** The risk that justified the whole spike turned out to be an
application-side pooling mistake, which is a much better position than a driver
we cannot fix.

### Transactor semantics (AC-3)

Arm A (4 connections, `BEGIN IMMEDIATE`), 30s soak: **31,106 counter commits,
22,036 pair transactions**, clean sweep, no watchdog dump.

The controls are what make this a measurement:

- **Arm B (deferred) FAILS** — lock upgrade returns `SQLITE_BUSY` regardless
of `busy_timeout`. `BEGIN IMMEDIATE` is load-bearing, not cargo-cult.
- **Arm C (`MaxOpenConns(1)`) HANGS** — `database/sql` pool starvation, not a
SQLite lock, reproduced in ~30 lines with no rela code. **Never serialize by
shrinking the pool**; use a normal pool plus an in-process write mutex.
- **Arm E: `RunTxRollbackTests` PASSES**, including `NoEventsOnRollback` and
`EventsDeliveredAfterCommit` — the suite fsstore and memstore deliberately do
not attempt.

| | fsstore | **sqlite (measured)** | pgstore |
|---|---|---|---|
| Serialization | in-process mutex | **in-process mutex + SQLite write lock** | cross-process |
| Rollback | none | **yes** | yes |
| Events on rollback | n/a | **withheld** | withheld |

### Performance (AC-5), 10k entities

Cold open **2.23ms**; steady-state single write **127µs**; bulk load 223ms
(44,749/sec). Cold open is the clearest win: fsstore parses and indexes all 10k
markdown files at startup, which is the residency that motivated the
investigation. The accepted ~1.5-1.9x modernc-vs-mattn write penalty is
irrelevant at 22µs/entity — the driver choice does not gate this work.

### Multi-process (AC-4)

`unique:` is enforced by an **untransacted** `ListEntities` scan
(`entitymanager/unique.go:82`; zero `.Tx(` sites in the package), so the race is
not even multi-process-specific. A multi-process SQLite backend inheriting
`reconcileDerivedSchemaIfSupported` as a no-op would have **no uniqueness
backstop at all**. Separately, SQLite's own constraints ARE cross-connection: 8
racers on one ID gave 1 winner, 7 `ErrConflict`, 0 other.

This is why single-process is **enforced by an advisory lock**, not merely
documented.

## What we are accepting

1. **Single-process only**, enforced at open. A second process must fail
loudly rather than corrupt quietly.
2. **No git-diffable markdown** on this backend. Content versioning is the
replacement, exactly as `docs/postgres-backend.md:194` already frames it for
pgstore. This is why versioning is *not optional* here.
3. **`synchronous=NORMAL`** — a crash can lose recently committed
transactions. A deliberate durability trade for a backend replacing git as the
history story, revisit if it proves wrong.
4. **No custom FTS5 tokenizers**, ever (modernc exposes no registration API).
5. **Network/sync filesystems unsupported** — and this one is **assumed, not
measured**: no network storage was available. Mitigation: `Open` already reads
back the real `journal_mode`, so refuse to start (or warn loudly) when it is not
`wal`. That converts a silent corruption risk into a clear error for the desktop
user who puts a project in iCloud.

## Prerequisite before any backend work

`appbuild` discovers pgstore capabilities by **concrete-type assertion**
(`derivedschema_postgres.go:23`, `userstate_postgres.go:27`,
`versionsweep_postgres.go:42,93`), and `StateKV` is not a store interface at
all. **These must be widened to interfaces first** — a third smart backend
cannot otherwise opt into versioning, state or derived-schema. Independently
valuable; easy to miss in an estimate.

## Not licensed by this decision

Multi-process SQLite, replacing pgstore for shared servers, FTS5 search
integration, SQL pushdown, or `ManifestSince`. Each is a separate decision.
Stage 2's effort is **unestimated until scoped**.

---
id: RES-BB8A86O
type: research
title: 'Time-series and ledger-style records in rela: lightweight support for non-graph data attached to entities'
summary: 'Add one append-only record store anchored to entities (records: in the schema), gated by the owner row, with aggregates that feed rollups; a ledger is a record type with a balanced-batch and reversal-only constraint. Not a TSDB or accounting package.'
status: done
---

## Problem

rela models data as typed entities and relations. Some applications built on
rela also have a smaller need for data that is not graph-shaped:

- **Time series:** readings, measurements or metrics attached to an entity.
Examples: a meter reading per asset, hours logged per project per day, a KPI
value per month, an uptime sample per system.
- **Ledger-style records:** append-only postings that must balance. Examples:
hour or budget allocations, stock movements, loyalty points, a simple cash book.

The goal is to let an application author add a few such use cases next to their
graph. The goal is **not** to turn rela into a time-series database or an
accounting package. The question is which small, composable primitive gives that
support and fits rela's storage, ACL and write-path rules.

## Context

### Why entities are the wrong unit

Every entity write pays for a markdown file (fs), an audit line, a search index
update, content versioning, automations and validation. A reading has no use for
any of these. At a few thousand rows per owner the fs backend, bleve and git
history all degrade. Entities are also mutable, while a ledger needs rows that
can never change.

Today the only "append-only" mechanism is the webhook `append_section` ingest
step (`internal/ingestspec/spec.go:86-98`). It appends a line of markdown to an
entity body. That is a workaround, not structured data.

### Side stores already solve the shape

rela has several stores that sit outside the graph and anchor to an entity ref.
They all follow one pattern:

- a narrow `Store` interface at the consumer;
- one backend per tier (file or KV, postgres, sqlite);
- a conformance suite (`commentstest`, `pilestest`, `leasetest`, `channelstest`);
- a service that checks the anchoring entity's row gate.

| Store | Anchor | Gate | Notes |
| --- | --- | --- | --- |
| comments | entity | `TargetReadGate.CanRead` plus `comment:read` (`dataentry/comments_wiring.go:135-149`) | travels with `rela.db` on sqlite |
| leases | entity family | `lease.Gate` over the gated reader, writes via write ACL (`appbuild/leases.go:94-130`) | cleanest gate template |
| channels | `ID` or `ID@face` | realtime access layer | append-only messages, retention, rename re-keying, tombstones; shared SQL via `sqlexec` |

`channels` is the closest analogue: an append-only, entity-anchored stream with
retention, owner rename and owner delete already handled. `internal/sqlexec`
lets one SQL implementation serve both postgres and sqlite (`sqlchannels`,
`sqlflowrun`).

### Aggregates

`store.Rollup` offers count, min, max, sum and latest, gated inside the backend
(`internal/store/rollup.go`). It only aggregates over **related entities**.
There is no avg, and `rollup.Compile` accepts sum over integers only, not
decimals (`internal/rollup/rollup.go:223-239`). The `Column` source mapping
(`rollup.go:55,73`) is the seam where another data source could plug in.

The CLAUDE.md rule on aggregates applies: gate the rows **before** folding, and
never cache a per-principal result across principals.

### Other building blocks

- `decimal` property type exists: exact text, parsed to `math/big.Rat`, scale up
to 30 (`internal/metamodel/decimal.go`). Ledgers need exactly this.
- Ingest: declarative webhooks (`ingest.Run`) and Lua write bindings. Neither
can write structured rows outside an entity today.
- No table or object property type; only lists of scalars.
- Frontend has no chart library. There is a Gantt view and dashboard tiles.

### Prior art outside rela

- **Salesforce Big Objects, Dataverse elastic tables:** a second, cheaper record
class next to the normal one, with fewer features (no triggers, limited queries,
no sharing rules of their own). This is the closest match to the intent here.
- **ERPNext / Frappe:** `GL Entry` and `Stock Ledger Entry` are ordinary
doctypes that become immutable after "submit". A cancel writes reversing rows.
Shows that a ledger needs very little beyond immutability and a balance rule.
- **TigerBeetle:** the ledger primitive reduced to accounts plus immutable
transfers with debit and credit, enforced at write time. Good model for the
minimum invariant set.
- **beancount / hledger:** plain-text journals with balance assertions. Fits
rela's files-in-git tier.
- **Prometheus, InfluxDB, TimescaleDB:** series identified by name plus labels,
time buckets, retention, continuous aggregates. Useful for the query shape; far
more than rela needs to build.

## Options

### A. Keep using entities, add small enablers

Model each reading or posting as an entity related to its owner. Add `avg`,
decimal `sum` and time-window filters to rollups.

- **Pros:** no new subsystem; works today with full ACL, views, search, history.
- **Cons:** per-write cost and volume ceiling stay; no immutability; ledger
balance can only be a soft validation; floods search, audit and git history.
- **Effort:** S. Worth doing regardless (decimal sum, avg), but it does not meet
the goal on its own.

### B. Record types: one append-only side store declared in the schema (recommended)

Add a `records:` section to the metamodel. A record type has:

- an **owner** entity type (the anchor), like comments and channels;
- typed **fields** from the existing property types (decimal, integer, date,
datetime, enum, string, an `entity` ref field for dimensions);
- a required **timestamp**;
- **append-only** semantics: no update, no delete, except retention and owner
deletion.

Sketch:

```yaml
records:
  meter-reading:
    owner: asset
    fields:
      value: { type: decimal, scale: 3, required: true }
      unit:  { type: enum, values: [kWh, m3] }
    retention: 5y
  hour-booking:
    owner: project
    fields:
      hours:  { type: decimal, scale: 2 }
      person: { type: entity, target: person }
```

Implementation follows the channels pattern: `internal/records` with a narrow
`Store`, a shared `sqlrecords` backend over `sqlexec` for postgres and sqlite,
and a JSONL backend for fs (one file per owner per month, so it diffs well in
git). Plus a `recordstest.RunAll` suite.

- **Reads:** list by owner and time range with paging; aggregate by owner(s)
with count, sum, min, max, avg, latest and optional time buckets (day, week,
month). Exposed through REST, MCP, Lua and the CLI.
- **ACL:** a record is readable when its owner row is readable (the `lease.Gate`
template), plus an optional `records:read` / `records:append` permission per
record type. Aggregates over many owners gate the owner set first, then fold.
Field-level `visible:` is out of scope for v1: a record type is visible or not
as a whole.
- **Writes:** an append through a records service that checks the owner write
gate and the field types. No automations, no audit entry per row (the stream
itself is the log), no search, no versioning.
- **Graph integration:** a rollup column may read from a record type (`latest`,
`sum`, `avg` over a window), so predicates, views, kanban and validations can
use "last reading" or "hours this month". This is the feature that makes it more
than a sidecar database.
- **Ledger as a flavour, not a second store.** A record type may declare
`ledger:`:

  ```yaml
  records:
    posting:
      owner: account
      ledger:
        amount: amount          # decimal field
        group_by: transaction   # postings sharing this key must sum to zero
        commodity: currency     # optional; zero-sum per commodity
        closed_before: lock_date  # optional; owner or project property
      fields:
        amount:      { type: decimal, scale: 2 }
        currency:    { type: enum, values: [EUR] }
        transaction: { type: string }
        memo:        { type: string }
  ```

The store accepts a **batch** append (one transaction) atomically, rejects it if
it does not balance, and offers a `reverse(transaction)` that appends the
negated postings. Balances are just the `sum` aggregate per owner. Account
hierarchy, cost centres and customers stay entities in the graph.

- **Pros:** one subsystem covers both needs; reuses property types, decimals,
the side-store pattern, `sqlexec` and the rollup seam; cheap writes; strong
ledger invariants enforced in one place; small schema surface; room to grow (see
"Growth path" below).
- **Cons:** a new store with three backends and a conformance suite; rollups
need a second source kind; one more concept for operators to learn.
- **Effort:** M for records plus aggregates, S for the ledger flavour on top,
S to M for a basic chart widget in the SPA.

### C. Two dedicated subsystems: a series store and a ledger store

A TSDB-style store (series name plus labels, downsampling, continuous
aggregates) and a TigerBeetle-style accounts-and-transfers store.

- **Pros:** each can be tuned for its job; richer features (labels,
downsampling, pending transfers).
- **Cons:** two of everything: backends, suites, ACL adapters, APIs, UI. Labels
duplicate what entity refs already give. Overshoots the "few smaller use cases"
goal.
- **Effort:** L.

### D. Integrate external systems only

Keep data in Prometheus, TimescaleDB or a beancount file and link to it with
`external_ref`, views or transforms.

- **Pros:** nothing to store in rela.
- **Cons:** rela's ACL cannot gate it; no rollups or predicates over the data;
desktop and fs users get nothing; each integration is custom.
- **Effort:** varies. Not needed as a fallback once B has its growth path:
postgres with TimescaleDB covers heavy workloads behind the same API.

## Recommendation

**Option B**, delivered in three slices, plus the cheap enablers from A.

1. **Records core.** `records:` in the schema, `internal/records` store with
shared SQL and JSONL backends, conformance suite, append and range-list over
REST, MCP, Lua and CLI, owner-row gating, owner rename and delete handling
copied from channels, retention.
2. **Aggregates and graph integration.** Bucketed aggregates; rollup columns
sourced from a record type; decimal `sum` and `avg` for rollups in general; a
small chart or sparkline on the entity page.
3. **Ledger flavour.** `ledger:` on a record type: balanced batch append,
reversal, optional period lock.

Tradeoffs accepted:

- Records are second-class on purpose: no relations, faces, search, versioning,
automations or field-level `visible:`. If a row needs those, it should be an
entity.
- No reports, tax logic, currency conversion or downsampled storage. Those
belong to views, Lua or an external tool.
- Batch append, downsampling and TSDB storage are not in the first slices, but
the design leaves room for each (see "Growth path").

### Growth path

Design is judged against postgres; the fs backend only has to be simple and
correct for one local user.

- **Batch append.** The append API takes a batch from the start (the ledger
needs it anyway). One gate check per owner per batch, one multi-row insert.
- **Downsampling.** A record type may declare `buckets: [hour, day]`. A job
maintains per-owner bucket rows (count, sum, min, max, last) and optionally
expires raw rows older than a horizon. This stays compatible with the
gate-before-fold rule because buckets never mix owners: the reader's owner set
is gated first, then their buckets are folded. `avg` comes from sum and count.
The constraint this puts on the design: record visibility must stay "visible if
the owner is", with no per-record gate, or buckets would fold hidden rows.
- **TSDB storage on postgres.** TimescaleDB is a postgres extension. A
hypertable is a normal table to SQL, so the `sqlrecords` backend can run on it
unchanged, and a postgres-only backend variant can use continuous aggregates as
the bucket implementation plus native compression and retention. Caveats:
compression, continuous aggregates and retention policies are under the
Timescale License rather than Apache 2 (fine to self-host, verify before relying
on it), and some managed postgres services (AWS RDS) do not offer the extension.
Native declarative partitioning by time (optionally with pg_partman) and BRIN
indexes are the license-free fallback. The extension stays optional:
rela-maintained bucket tables work on plain postgres and sqlite.

### Open questions for the design ticket

- Are records project content (travel in git on fs and in `rela.db` on sqlite,
like comments) or node-local state? The comments rule suggests content.
- Should appending a record be able to trigger a flow (for example threshold
alerts), and through which event? Not via synchronous automations.
- Does remote sync need to carry records?
- Should a record type with `ledger:` require its owner type to be an entity
type marked as an account, or stay generic?

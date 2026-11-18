---
id: RES-9IOGBV3
type: research
title: Point-in-time reads over version history
summary: Use ?as_of=, shadow entities/relations with snapshot CTEs on the store's execution seam, carry the instant on ctx, gate with today's ACL.
status: done
---

## Problem

An auditor asks: "What did the statement of applicability look like on 31
December?" rela keeps per-face version history on PostgreSQL and SQLite, but
every read surface reads the live tables. We need a request to read a world as
it stood at an instant, on every read surface, read-only and gated like a live
read.

## Context

- Versions: `entity_versions` (per `(entity_id, face)` lineage, ops create/update/rename/delete/purge) and `relation_versions` (per `rel_record_id`). Create and update are captured by a debounced sweep; `created_at` is the capture time, which trails the edit by the idle window, and by much more when no sweep ran (a CLI edit while no server is up). Rename and delete are captured synchronously.
- Live rows carry `updated_at`, the time of the last write. A live row whose `updated_at` is at or before T, and whose lineage has no version after T, is exactly the state at T.
- Reads in pgstore and sqlitestore are SQL text that names the `entities` and `relations` tables (point reads, lists, header pages, graph queries with recursive CTEs, relation reads, roll-ups).
- Worlds: the request's world is a `worldHandle` bound once by `attachWorld`; routes opt in through `worldCapablePath`. Writes are already refused under `?world=`.
- The ACL gate, the visibility readers, the tracer and Lua readers all read through the store handle built at assembly.
- Search reads a separate index (bleve, or the pg/sqlite in-DB search over `search_text`).

## Options

### Instant on the request

1. `?as_of=<RFC 3339>` query parameter, independent of `?world=`. Composes with every world; the SPA keeps it in the route query like `world`. Pros: one orthogonal axis; the world name stays a config key. Cons: one more parameter on the allowlist.
2. World qualifier `published@2025-12-31`. Pros: one parameter. Cons: overloads a config key with data; every place that compares world names must parse it; date-only form is timezone-ambiguous.

### Store mechanism

A. Rewrite every read query against the version tables. Large diff, duplicated
SQL, drifts from the live queries.

B. Shadow the tables with CTEs. Prefix each read statement with `WITH entities
AS (<snapshot at T>), relations AS (<snapshot at T>)`. A CTE name takes
precedence over a table name in both PostgreSQL and SQLite, so every existing
read statement (including the graph-query builder) answers from the snapshot
unchanged. The snapshot reads the live table through a schema-qualified name
(`"<schema>".entities`, `main.entities`) so it never refers to itself. The view
is a second store value whose execution seam (pg `DBTX`, sqlite `querier`)
rewrites the statement and refuses `Exec` and `Begin`, so no write can run. The
instant is a Go-formatted literal, so no placeholder renumbering.

C. Materialize the snapshot into temp tables per request. Correct but copies the
whole history per request.

### Snapshot rule (both backends)

For each entity lineage `(id, face)`:
- the live row, if `updated_at <= T` and no version of the lineage has an edit time after T;
- else the latest version with edit time `<= T`, unless that version is a `delete` or `purge`, or is superseded by a rename or face conversion at or before T.

Edit time is a new `content_at` column: the source row's `updated_at` for a
swept or captured-now version, the write time for a synchronous rename or
delete. Legacy rows fall back to `created_at`. Relations follow the same rule
per `rel_record_id`, plus both endpoints must exist in the entity snapshot.

### Routing the instant to every reader

i. Swap the store per request in every handler: many call sites, one missed site
serves live data. ii. A routing decorator installed at assembly, below every
consumer (ACL graph, visibility, tracer, Lua). Reads consult `ctx` for the
instant and go to the backend's as-of view; writes under an instant are refused.
Backends without the capability return `store.ErrAsOfUnsupported`.

### Access control at T

- Today's policy and today's identity (roles, memberships resolved at the request edge), applied to the rows as they stood at T: row and field conditions read the snapshot because the gate reads through the same routed store.
- The ACL as it stood at T: the policy is config, not versioned in the store, so it cannot be reconstructed; and it would let a principal who has lost access read the past.

## Recommendation

Option 1 (`?as_of=`), mechanism B, routing ii, and today's ACL applied to the
snapshot. Search under an instant matches linearly over the snapshot's headers
(the index holds only live text). Routes join an `asOfCapablePath` allowlist one
by one, like worlds, so a route that reads another source (search index,
document render cache) cannot serve live data under an instant. fsstore and
memstore answer with `ErrAsOfUnsupported`.

Accepted tradeoffs: edits inside one sweep debounce window merge into one
version, so an instant inside that window sees the merged state; purged history
reads as absent; as-of reads scan the version tables of the types they read
(audit reads are rare; calls per page stay flat).

Delivery: PR 1 store (column, views, router, conformance suite); PR 2 data-entry
(`?as_of=`, write refusal, route coverage and world-follow tests for each view);
PR 3 SPA and docs.

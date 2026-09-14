---
id: TKT-S1EVV7
type: ticket
title: Split the SQLite connection from the store, add project_files and a migration ladder
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Config living in the database means config must be readable *before* the store
exists. Separating the SQLite connection from the store is what makes that
ordering work, and it is a refactor rather than a redesign because
`sqlitestore.OpenContext` already decomposes along exactly this line: acquire
the sidecar lock → `sql.Open` → `init(ctx)` (PRAGMA verify + schema).

Three changes:

- Extract `Connect(ctx, Options)` returning a usable handle after `init`.
- `sqlitestore.New(conn, opts...)` mirroring `pgstore.New(db DBTX)`
(`internal/store/pgstore/pgstore.go:196`), which already takes an injected pool
with appbuild owning and closing it. Same shape, second backend.
- Add the `project_files` table, bump `schemaVersion` to 2, and add the first
rung of the migration ladder.

## The ladder is required, not optional

`schemaSQL` is `CREATE TABLE IF NOT EXISTS`, which is a silent no-op against an
existing table of a different shape — the failure lands later, at the first
query, on user data. `internal/cli/db_sqlite.go` currently documents the ladder
as deliberately absent because no shipped database exists yet; adding a table is
exactly the event its comment names as the trigger. `runDBMigrate` and
`runDBStatus` stop being report-only stubs.

## Table

```sql
CREATE TABLE IF NOT EXISTS project_files (
	path       TEXT PRIMARY KEY,   -- 'schema.yaml', 'scripts/foo.lua'
	content    BLOB NOT NULL,      -- BLOB: custom/ and apps/ carry fonts and images
	updated_at TEXT NOT NULL
) STRICT;
```

Flat path keys, no directory rows — listing is a prefix scan, which is all any
consumer needs.

## Constraints

- `plimsoll` — `Store` carries `max-methods=53 / max-exported-methods=32`.
`Connect`/`New` must *replace* `Open`/`OpenContext` rather than add to them, so
the exported count does not rise.
- The DSN-PRAGMA rule from DEC-LFSYNY is untouched: PRAGMAs stay in the DSN so
they apply to every pooled connection, never `db.Exec`.
- Build tags: nothing but the `sqlite` build may link `modernc.org/sqlite`; CI
asserts this via `go list -deps`.

## Acceptance

- Conformance suite (`storetest.RunAll` + fuzz) still passes.
- A database at version 1 migrates forward; one from a newer binary is still
refused rather than mis-read.
- `just build-check-tags` clean.

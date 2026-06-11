---
audience: intermediate
id: GUIDE-postgres-backend
order: 13
status: published
summary: Run rela-server and the CLI against PostgreSQL instead of markdown files
title: PostgreSQL Backend
type: guide
---

By default rela stores entities and relations as markdown files and
indexes search with an in-process bleve index. A separate **PostgreSQL
build** stores the same data — entities, relations, attachments, and the
search index — in a PostgreSQL database instead. It is selected at compile
time with the `postgres` Go build tag and shipped as separate binaries:
`rela-postgres` and `rela-server-postgres`.

Use it when you want a real database behind rela (durable, concurrent,
SQL-queryable) rather than a directory of markdown files — including
deployments with **multiple server processes** sharing one database (see
[Multiple writers](#multiple-writers)).

## What still lives on disk

PostgreSQL backs **data only**. The project's schema and configuration
are still read from the filesystem, exactly as in the default build:

- `metamodel.yaml` — the entity/relation schema.
- `templates/` — optional entity/relation templates.
- `.rela/` — the per-machine cache and audit log.

So a PostgreSQL deployment still points at a project directory (via
`--project` or the working directory); the database URL is **additional**
configuration, not a replacement for the project directory.

## Configuring the connection

The connection string is read from the `RELA_DATABASE_URL` environment
variable. It is **env-only** — there is deliberately no `--database-url`
flag, because a credential-bearing connection string passed on the command
line is visible in `ps` output, shell history, and process listings.

```bash
export RELA_DATABASE_URL='postgres://user:password@db.internal:5432/rela?sslmode=require'
rela-server-postgres --project /srv/rela/project --bind 0.0.0.0 --port 8080
```

On a connection or parse failure, rela surfaces a sanitized error — the
password is **not** echoed.

For production, set `sslmode=require` (or stricter — `verify-full` with a
CA) so the connection is never silently unencrypted.

## Schema and migrations

On first start the PostgreSQL build creates its schema automatically
(an `entities`, `relations`, `attachments`, and `schema_version` table,
plus the `pg_trgm` extension for substring/fuzzy search). Migrations are
embedded in the binary and applied idempotently on every start — they run
in a single transaction under an advisory lock, so concurrent starts are
safe — and upgrading is just deploying a newer binary and restarting.

The connecting role needs privileges to create the `pg_trgm` extension on
first run (typically a superuser, or have an administrator run
`CREATE EXTENSION pg_trgm;` once in the target database beforehand).

### Applying migrations explicitly

If you would rather apply the schema as a separate, controlled step
(for example with a more-privileged role at deploy time, or to gate a
release in CI) the `rela db` command group does this without starting a
server:

```bash
rela db status    # report current vs expected schema version; non-zero exit if behind
rela db migrate   # apply pending migrations; a no-op when already current
```

`rela db status` is read-only and makes no changes — handy as a CI gate.
Both commands read the connection string from `RELA_DATABASE_URL` and
exist only in the PostgreSQL build. Auto-migrate
on startup remains the default, so no explicit step is required for the
common single-server case.

rela's tables are created in the connection's default schema (typically
`public`). Point rela at a database it owns; if you share a schema with
another application, rela's tables sit alongside it.

## Search

In the PostgreSQL build, search runs **in the database** (a `tsvector`
GIN index for ranked full-text plus `pg_trgm` for substring and fuzzy
matching) — there is no bleve index. Text search matches the same fields
as the default backend: entity ID, content, and string-valued properties.

## Multiple writers

You can run **multiple processes against one database** — for example several
`rela-server` instances behind a load balancer, or a `rela-server` plus an
occasional `rela` CLI write. Each process sees the others' changes, so the UI
stays live across servers: an entity created, updated, or deleted on one server
appears in browsers connected to another, without a manual refresh.

What you need to know to run it:

- All processes that should share live updates must point at the **same database
  and schema** — which they do when they serve the same project.
- Each process opens **one extra connection** to receive change notifications.
  If it can't, the process still works normally; only the live cross-server
  updates are unavailable (a warning is logged).
- Live updates cover **entity** create/update/delete. Relation and attachment
  edits are reflected on the next page load rather than pushed live.

## Other scope notes

- The desktop app (`rela-desktop`) is filesystem-only; there is no PostgreSQL
  desktop build.
- There is no automatic migration of an existing filesystem project into
  PostgreSQL; the database starts empty.

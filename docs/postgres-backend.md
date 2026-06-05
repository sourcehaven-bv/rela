<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# PostgreSQL Backend

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

The PostgreSQL build supports **multiple processes writing to one database**
— for example several `rela-server` instances behind a load balancer, or a
`rela-server` plus an occasional `rela` CLI write. Each process learns about
the others' committed writes through a PostgreSQL `LISTEN/NOTIFY` change feed:

- On each committed write, the writing process emits a notification carrying
  the changed entity's identity. Other processes receive it on a dedicated
  listening connection and turn it back into a change event — the same event
  an in-process write produces. This drives cross-server **live-reload**: a
  browser connected to one server reflects an entity created, updated, or
  deleted on another server.
- Because notifications are not durable (a process that is restarting or
  briefly disconnected misses them), each process also runs a periodic
  catch-up that reconciles from the monotonic sequence number every row
  carries. The feed is therefore self-healing: a missed notification is
  recovered on the next catch-up.

Practical notes:

- All processes that should see each other's writes must point at the **same
  database and schema** (they do — it's the same project's data). The feed is
  scoped to the schema, so unrelated projects in other schemas don't interfere.
- Each process holds **one extra long-lived connection** for listening. If
  that connection can't be established, the process still runs (and its own
  writes still drive its own live-reload) — only cross-process events are
  unavailable, and a warning is logged.
- The live feed covers **entity** changes (create/update/delete). Relation and
  attachment edits are not pushed to browsers live (they weren't in the
  single-server case either); a page reload reflects them.

## Other scope notes

- The desktop app (`rela-desktop`) is filesystem-only; there is no PostgreSQL
  desktop build.
- There is no automatic migration of an existing filesystem project into
  PostgreSQL; the database starts empty.

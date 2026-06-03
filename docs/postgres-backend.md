<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# PostgreSQL Backend

By default rela stores entities and relations as markdown files and
indexes search with an in-process bleve index. A separate **PostgreSQL
build** stores the same data — entities, relations, attachments, and the
search index — in a PostgreSQL database instead. It is selected at compile
time with the `postgres` Go build tag and shipped as separate binaries:
`rela-postgres` and `rela-server-postgres`.

Use it when you want a single server process backed by a real database
(durable, concurrent reads, SQL-queryable) rather than a directory of
markdown files.

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

The connection string is read from, in order of precedence:

1. The `--database-url` flag (`rela`, `rela-server`).
2. The `RELA_DATABASE_URL` environment variable.

```bash
export RELA_DATABASE_URL='postgres://user:password@db.internal:5432/rela?sslmode=require'
rela-server-postgres --project /srv/rela/project --bind 0.0.0.0 --port 8080
```

Prefer the environment variable for secrets: a connection string passed
as a flag is visible in `ps` output and shell history. On a connection
or parse failure, rela surfaces a sanitized error — the password is
**not** echoed.

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
Both commands read the connection string from `--database-url` /
`RELA_DATABASE_URL` and exist only in the PostgreSQL build. Auto-migrate
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

## Scope and limitations

This build targets a **single server process owning the database**:

- Live-reload / change events are in-process only. Running more than one
  writer against the same database is not supported yet — a second writer's
  changes would not be observed by the first process's search index.
- The desktop app (`rela-desktop`) is filesystem-only; there is no
  PostgreSQL desktop build.
- There is no automatic migration of an existing filesystem project into
  PostgreSQL; the database starts empty.

Every row carries created/updated timestamps and a monotonic sequence
number, so cross-process change propagation can be added later without a
schema migration.

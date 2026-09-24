<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# SQLite Backend

By default rela stores entities and relations as markdown files. The
**SQLite build** stores the same data — entities, relations and
attachments — in a single embedded database file instead, with no database
server to run. It is selected at compile time with the `sqlite` Go build tag
and shipped as `rela-sqlite` and `rela-server-sqlite`.

It sits between the other two backends. The filesystem build keeps everything
in git-diffable markdown but holds the whole graph in memory and rebuilds its
search index at startup. The PostgreSQL build gives you indexed queries and
multi-process deployment, at the cost of running a database. SQLite gives you
the indexed queries without the server — for **one process at a time**.

Use it for a desktop install or a single-server deployment. If you need two
processes against one dataset, use PostgreSQL; this backend will refuse to
start rather than let that happen.

## What still lives on disk

SQLite backs **data only**. The project's schema and configuration are read
from the filesystem exactly as in every other build:

- `schema.yaml` — the entity/relation schema.
- `templates/`, `scripts/`, `acl.yaml` — operator-authored configuration.
- `.rela/` — the per-machine cache, the audit log, and the database itself.

So a SQLite deployment still points at a project directory. The database is
created at `.rela/rela.db` the first time you open the project; there is no
connection string to configure.

## Single process, enforced

Opening the project takes an exclusive lock. A second process gets a clear
refusal naming the process that holds it:

```text
sqlitestore: another process is using /path/.rela/rela.db; this backend is
single-process by design. Stop the other process, or use the PostgreSQL
build for a multi-process deployment
```

This is a refusal rather than a warning for a specific reason. rela enforces
`unique: true` properties with a scan-then-write check that is not wrapped in a
transaction. With one writer that window is narrow and harmless. With two
processes there is **no backstop at all** — both can pass the check and both
can write, and nothing reports it. The PostgreSQL build closes that with a
database-level unique index; this build closes it by ensuring there is only
ever one writer.

The lock is released when the process exits, including on a crash: it is an
OS-level advisory lock on a sidecar file, not a flag written into the database.

## Not supported on network or sync filesystems

Do not put a SQLite-backed project on iCloud Drive, Dropbox, OneDrive, or an
SMB/NFS share. SQLite's write-ahead log needs shared memory that those
filesystems do not provide, and the file locking they report cannot be trusted.

rela checks this at startup and **refuses to open** rather than corrupting the
database later:

```text
sqlitestore: WAL could not be enabled (journal_mode="delete") for
/path/.rela/rela.db — this usually means the file is on a network or
file-sync filesystem (iCloud, Dropbox, SMB), where SQLite is not safe
```

Keep the project on local storage. If you want it on more than one machine,
that is what the PostgreSQL build and `rela sync` are for.

## What you give up

Compared with the **filesystem** build:

- **No markdown files.** Entities live in the database, so you cannot `grep`
  them, hand-edit them, or review a change as a diff.
- **No git history.** The filesystem build gets version history for free
  because every entity is a file in your repository. The SQLite build replaces
  that with built-in content versioning, the same way the PostgreSQL build
  does — a time machine you can list, diff and restore from, stored in the
  database rather than in your repository. What you give up is the *review*
  workflow: history is queryable, but there is no pull request to read.

Compared with the **PostgreSQL** build:

- **One process.** No shared server, no multi-tenant deployment.
- **No cross-process change feed**, because there is no second process.
- **No shared runtime state.** Settings, the render cache and the operator's
  logo live under `.rela/` rather than in the database. That is deliberate:
  node-local state is only a problem when several processes serve one project,
  and this backend is single-process by construction.

Content versioning, version purge and comments are NOT in that list any more —
both backends implement the same contract, and one shared conformance suite
holds them to it.

Comments are the one thing that went into the database *despite* the
single-process argument above, and the difference is worth naming. Settings and
a render cache are about the machine; a comment is about the content. An
operator who copies `rela.db` expecting "the project" would otherwise find every
entity present and every remark on them left behind.

## What you get

- **Fast startup on large projects.** The filesystem build parses and indexes
  every markdown file before it can serve a request; SQLite opens the file and
  reads what it needs. On a 10,000-entity project the difference is
  milliseconds versus seconds.
- **Real transactions.** A failed write is rolled back completely, and nothing
  observes a change that did not commit — the same guarantee the PostgreSQL
  build makes, which the filesystem build cannot.
- **One file to back up.** Copy `.rela/rela.db` while rela is not running.
- **Queries answered in the database.** Lists, counts, query scopes and the
  ACL's relation gates run as one SQL statement each, as on PostgreSQL, and
  listings never load markdown bodies they will not show. The
  build also derives the same indexes from your static queries and lists. It
  creates `rela_derived_query__…` and `rela_derived_list__…` indexes at
  startup, so a list page sorted ascending reads one index range instead of
  sorting the whole type. A descending sort still sorts its last key. The
  PostgreSQL guide's "Derived schema" section describes which queries qualify;
  the rules are the same here.

## Derived indexes

Startup converges the derived indexes on what `data-entry.yaml` declares:
missing ones are created, undeclared ones are dropped. A missing
`data-entry.yaml` is valid. An invalid or unreadable one skips the reconcile
and keeps the existing indexes, because dropping them on a half-read file would
be worse than leaving them stale.

```bash
rela db reconcile           # create/drop derived indexes to match the configuration
rela db reconcile --dry-run # show what WOULD change; non-zero exit if anything would
```

`rela db reconcile` opens the database, so it cannot run while a server has the
project open. A dry run changes nothing: with no database yet it says so and
exits 0, and it refuses a database whose schema is older than the binary
instead of migrating it. `unique: true` does not produce an index on this
build: the single-writer lock above is what makes the application-level check
sound.

## Migrating between backends

There is no automated migration between backends yet. The data lives in
different places and nothing converts one into the other, so choose a backend
before you have data you care about — or move it by exporting and re-creating.

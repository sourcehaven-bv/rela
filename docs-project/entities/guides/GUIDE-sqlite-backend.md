---
audience: intermediate
id: GUIDE-sqlite-backend
order: 14
status: published
summary: Run rela against an embedded SQLite database — one file, no server, single process
title: SQLite Backend
type: guide
---

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
- **No git history.** This is the sharpest trade. The filesystem build gets
  version history for free because every entity is a file in your repository.
  The PostgreSQL build replaces that with built-in content versioning — a time
  machine you can list, diff and restore from. **The SQLite build has neither
  yet**: content versioning is planned but not implemented, so today this
  backend keeps no history of past edits. If an audit trail matters to you,
  use one of the other two.

Compared with the **PostgreSQL** build:

- **One process.** No shared server, no multi-tenant deployment.
- **No cross-process change feed**, because there is no second process.
- **No content versioning, no version purge, no shared runtime state.**

## What you get

- **Fast startup on large projects.** The filesystem build parses and indexes
  every markdown file before it can serve a request; SQLite opens the file and
  reads what it needs. On a 10,000-entity project the difference is
  milliseconds versus seconds.
- **Real transactions.** A failed write is rolled back completely, and nothing
  observes a change that did not commit — the same guarantee the PostgreSQL
  build makes, which the filesystem build cannot.
- **One file to back up.** Copy `.rela/rela.db` while rela is not running.

## Migrating between backends

There is no automated migration between backends yet. The data lives in
different places and nothing converts one into the other, so choose a backend
before you have data you care about — or move it by exporting and re-creating.

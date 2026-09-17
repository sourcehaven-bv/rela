---
id: TKT-OGTVJW
type: ticket
title: 'Database-backed comment stores: pgcomments and sqlitecomments over an injected pool'
kind: enhancement
priority: medium
effort: l
status: review
---

## Description

The follow-up ticket named in TKT-FIO205 and RR-JT560T: `pgcomments` and
`sqlitecomments`, the two database-backed `comments.Store` backends that stage 1
designed the interface for but deliberately did not ship.

Today `internal/appbuild/comments.go` wires `filecomments` **unconditionally**,
in every build. Its own doc comment says why that was right at the time and
names the condition that ends it:

> Backend choice is deliberately NOT build-tagged today: the file backend is
> correct for every current build (fs, memory, sqlite are all single-process,
> and the postgres build's multi-process concern arrives with pgcomments in the
> follow-up ticket).

That condition has arrived. On the postgres build, entities, relations,
attachments, search and `state.KV` are all in the database, but comments land in
each node's own `.rela/comments/*.yaml`. `docs/postgres-backend.md` documents
several `rela-server` processes against one database, so under that topology a
comment posted through one node is invisible to the others, and `filecomments`'
own doc concedes the ground: "A cross-process writer is out of scope for this
tier, exactly as it is for fsstore."

This is the same defect class TKT-VC27L3 fixed for `state.KV`, with the same
fix: put the data in the database that is already the source of truth, in the
tenant's schema, so schema-per-tenant scopes it for free.

## Scope

### The backends

Two new packages implementing `comments.Store` unchanged — **no interface
change**, which is the constraint TKT-FIO205 set and `commentstest.RunAll` is
what makes it cheap:

- `internal/comments/pgcomments` — PostgreSQL, one row per comment.
- `internal/comments/sqlitecomments` — SQLite, same row shape.

Both take an **injected handle**, never a DSN, matching `pgstore.New(db DBTX)`.
Neither depends on `internal/store`: a comment is not graph content and these
packages have no reason to know a store exists.

The sqlite build takes its comments from `rela.db` (not `.rela/comments/`),
following the versioning precedent (TKT-4NU9ZD) rather than the `state.KV` one
(TKT-L1A3PH): commentary must travel with the rows it annotates, so a shipped
database file carries its own comments. `state.KV` stays on the filesystem for
sqlite, unchanged.

### `pgstore.Open` loses its DSN form

`pgstore.Open(ctx, dsn)` currently parses the DSN, builds the pool, migrates,
constructs the store and the search backend, and starts the listener — a
composition root inside the store package. It returns the store, the searcher
and a closer, but **not the pool**, so the real composition root has no handle
to inject into a third consumer.

`Open` therefore takes a `DBTX` instead of a DSN, and `appbuild`'s postgres
recipe owns pool construction and `Migrate` — which is what CLAUDE.md already
says is true ("appbuild owns/closes the pool"). The store, the in-database
search backend and `pgcomments` then become three equal consumers of one
injected pool, which is what `NewSearchBackend(pool)` already is today.

The DSN-taking form is **deleted**, not kept as a convenience wrapper: two entry
points would leave the next backend free to pick the one that hides pool
ownership again. Test call sites move to a shared helper.

The listener keeps its own dedicated connection built from the DSN (by design —
a slow `LISTEN` must not starve query traffic), so the DSN stays a parameter
where the listener is started.

### Wiring

`internal/appbuild/comments.go` becomes per-build, selecting the backend the way
every other backend choice is made. `filecomments` remains correct for the fs
and desktop tiers and is untouched there.

## Out of scope

Threading, versioning and search indexing of comments stay permanently decided
against (TKT-FIO205). No new anchor kinds. No change to the HTTP surface, the
ACL permissions, or the `comments:` metamodel block. No migration of existing
`.rela/comments/` YAML into a database — an operator adopting postgres starts
with an empty comment store, as they do for every other table.

## Acceptance criteria

1. `pgcomments` and `sqlitecomments` each pass `commentstest.RunAll` with no
change to the `comments.Store` interface.
2. Concurrent adds to one target all survive on both backends, enforced by the
database rather than by a process-local mutex.
3. On the postgres build, a comment written through one process is visible to a
second process against the same database.
4. Comment rows live in the tenant's schema, so two schemas on one database do
not see each other's comments.
5. `pgstore.Open` takes an injected handle; no production or test call site
passes a DSN to it, and the postgres recipe injects one pool into the store, the
search backend and `pgcomments`.
6. The sqlite build stores comments in `rela.db`; `state.KV` stays on the
filesystem there.
7. The default and memory builds still use `filecomments`, and a build with no
`comments:` block still creates no storage and serves no routes.
8. `just arch-lint` passes: neither new package depends on `internal/store`.

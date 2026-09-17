---
id: TKT-4LG36M
type: ticket
title: Add comments.Store.Get so a single-comment read stops pulling the whole thread
kind: enhancement
priority: low
effort: s
status: planning
---

## Description

Deferred from TKT-OGTVJW as RR-1T6I3M.

`Service.Get` calls `List` and linear-scans in Go to find one comment. Every
`Update` and `Delete` authorization check therefore pulls up to `MaxPerTarget`
rows, with their bodies, to read one `author` field.

That is the per-row-lookup shape CLAUDE.md's collection-reads rule exists to
prevent ("a list, search, kanban or scope pipeline reads `store.EntityHeader`
rows and never a body it will not render").

## Why it looks worse than it is, and why it is still worth doing

It predates the database backends and was the only sensible option when
`filecomments` was the sole implementation: a thread is one YAML document there,
so reading one comment and reading all of them cost exactly the same. The
Go-side scan was free.

That stopped being true when two backends arrived that can page. On postgres and
sqlite this is now a real read amplification on the write path, bounded by
`MaxPerTarget` (500) rather than by anything the request asked for. It is not
urgent — a comment body is capped at 16 KiB and threads are usually short — but
it is a cost that only grows with adoption and the fix is small.

## Why it was not done in TKT-OGTVJW

Two reasons, both about scope rather than merit:

- AC1 of that ticket froze the interface ("no interface change, which is the
constraint TKT-FIO205 set"), and this is an interface change.
- Doing it there would have obliged all four backends and the conformance suite
to move in the same commit as two critical data-loss fixes. Bundling an
optimization with that is the wrong risk profile.

## Approach sketch

Add `Get(ctx, target, id) (Comment, error)` to `comments.Store`, returning
`ErrNotFound` when absent — the same contract `Update` and `Delete` already use
for a missing row, so there is no new error shape.

- **pgcomments / sqlitecomments**: a `WHERE target_key = ? AND id = ?` single-row
read, served by the existing `PRIMARY KEY (target_key, id)`. No new index.
- **filecomments / memcomments**: read the thread and pick, exactly as
`Service.Get` does today. Costs them nothing, since a thread is one document.
- **commentstest**: a `RunGetTests` block, including the not-found case and the
per-face case (a comment id must not resolve across faces).

Consider whether `Service.Get`'s existing callers all want the full comment or
only the author — if only the author, the narrower read is cheaper still, but do
not shape the interface around one caller.

## Acceptance criteria

1. `comments.Store` gains `Get`, and all four backends implement it.
2. The database backends serve it as a single-row read, not a thread read.
3. `Service.Get` uses it, so `Update` and `Delete` no longer list a whole thread
to authorize one comment.
4. `Get` on an absent id returns `comments.ErrNotFound`.
5. `Get` is face-scoped: an id present under `id@draft` must not resolve through
the default face.

---
id: TKT-JZY2PM
type: ticket
title: Comment edits stamp UpdatedAt on the database backends but not on file/memory
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

Found while adding the `Get` conformance suite for TKT-4LG36M. A new assertion
that an edited comment carries an edit time passed on `pgcomments` and
`sqlitecomments` and failed on `filecomments` and `memcomments`.

`comments.Store.Update` is implemented four ways:

- **pgcomments** — `SET body = $3, resolved = $4, updated_at = $5` with
`time.Now().UTC()`.
- **sqlitecomments** — the same, formatted through `timeFmt`.
- **filecomments** — sets `Body` and `Resolved`, never `UpdatedAt`.
- **memcomments** — sets `Body` and `Resolved`, never `UpdatedAt`.

So the same edit produces a different record depending on which backend is
compiled in.

## Why it matters

`Comment.UpdatedAt` is serialized to the API as `updated_at` with `omitzero`, so
the field is simply **absent** from every response on the default build and
**present** on a postgres or sqlite one. A client cannot tell "this comment was
never edited" from "this backend does not record edits", which is precisely the
ambiguity `omitzero` was chosen to express.

Nothing renders it today — the Vue SPA does not read `updated_at` — so there is
no user-visible defect yet. That is what makes this worth fixing now rather than
after something depends on it: the first feature to show "edited" would work in
a postgres deployment and silently do nothing on the filesystem tier.

It is also the interface contract drifting unobserved. `Store.Update`'s doc says
only that author, created-at and anchor are immutable; it does not say whether
an edit is timestamped, so all four backends are arguably compliant and the
suite never asked.

## Approach

Stamp `UpdatedAt` in `filecomments.Update` and `memcomments.Update`, matching
the database backends' `time.Now().UTC()`.

Consider instead having `Service.Update` set it once and pass it down, so the
value is minted in one place rather than four — the same reasoning that makes
`Add` take a server-written `CreatedAt` from the service rather than letting
each backend mint its own ("implementations persist them as given rather than
minting their own, so the values in an audit trail and the values stored
agree"). That is the more consistent shape, and it is a `Store.Update` signature
change, so decide before implementing.

Then tighten `commentstest`: `RunGetTests` currently asserts only that `Get` and
`List` agree about `UpdatedAt`, deliberately not whether it is set. Replace that
with a positive assertion in `RunUpdateTests` once the backends agree.

## Acceptance criteria

1. An edit stamps `UpdatedAt` on all four backends.
2. A comment nobody edited still reports a zero `UpdatedAt`, so `omitzero`
keeps meaning "never edited".
3. `UpdatedAt` comes back in UTC from every backend, as `CreatedAt` already
does.
4. `commentstest` pins 1-3 for every backend, so a fifth cannot drift.

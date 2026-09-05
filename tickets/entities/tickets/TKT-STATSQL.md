---
id: TKT-STATSQL
type: ticket
title: Store runtime state in the database rather than in files beside it
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Runtime state — the document render cache, user settings, the operator's logo
and theme, the CalDAV alias table — lives in files under `.rela/` on this
backend. Those sit **beside** the database rather than inside it, so a shipped
single file would arrive with its palette, logo and settings silently left
behind.

`internal/state/statesql` is `configsql`'s sibling and deliberately the same
shape: it takes a `*sql.DB` the caller already opened, touches nothing of the
entity graph, and is not part of a storage backend. The sqlite recipe builds
both from the one handle it owns.

## Why the motivation differs from postgres

The postgres backend moved this state into the database because several
`rela-server` processes share one, and node-local files meant an uploaded logo
was served by exactly one of them. SQLite is single-process, so that is not the
problem here — being **outside the file** is.

Worth stating, because copying pgstore's rationale would have suggested this
was unnecessary on a single-process backend.

## No compile-time conformance, on purpose

`statesql` does not import `internal/state`: that package **wraps** it
(`ValidatedKV` applies the key rules both backends must share), so naming it
would invert the dependency.

`state/statetest` proves conformance instead, and is the stronger check — it
pins *behavior*: a missing key is `ErrNotExist`-compatible, delete is
idempotent, an empty value round-trips distinctly from an absent one. A
compile-time assertion catches none of those.

## Bypass fixed along the way

`reconcileDerivedSchemaIfSupported` read `data-entry.yaml` straight off the
filesystem, bypassing the layering entirely. On a packaged project carrying
that config in its database it would find nothing and **silently drop every
derived static-query index**, with no error to explain the missing indexes. It
reads through the config seam now, and its no-op twin took the same signature.

## Acceptance

- `state/statetest.RunAll` passes, wrapped in `ValidatedKV` exactly as the
  wiring site wraps it, so key rejection exercises the production composition.
- An oversize value is REJECTED rather than stored short — a silently
  truncated cached render would be served as if valid.
- `state_kv` is created on fresh databases and by a v2→v3 migration rung, from
  one shared DDL constant so the two shapes cannot drift.
- Verified on a real project: `.rela/rela.db` holds `entities`, `relations`,
  `attachments`, `project_files` and `state_kv` at schema version 3.

## Known remainder

`.rela/audit/` and `.rela/search/` are still directories beside the database.
The search index is derived and rebuilds on open, so it is arguably fine to
leave; the audit log is not derived and needs its own decision.

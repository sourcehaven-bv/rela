---
id: TKT-YNKKRQ
type: ticket
title: fs-to-sqlite migration command
kind: enhancement
priority: medium
effort: l
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Description

Add a command that copies an fs project into a sqlite store (`.rela/rela.db`):
every entity, relation and attachment. Integration sync (FEAT-XYQMUB) needs
store-level history, which bare fs does not have. Today an fs user can only
switch by exporting and re-creating their data (`docs/sqlite-backend.md`,
"Migrating between backends").

Also decide whether `rela-desktop` gets a sqlite build. It is fs-only today, so
desktop users could not use integrations.

## Scope

- In: fs → sqlite, refusing a non-empty target, verification of counts.
- Out: sqlite → fs, postgres targets, keeping both copies in sync.

## Notes

- A raw-store write like `rela dev seed`: operator shell, attributed, one
audit record.
- Must not break `go list -deps` rules: only the `sqlite` build links
`modernc.org/sqlite`.

## One engine with `db load --data`

develop added `rela db load --data` (TKT-FGIWPE), a second markdown-to-SQLite
import. Both now run `fsimport.Copy`. `appbuild.ImportMarkdownData` keeps its
in-place, one-transaction model and `--force`, and builds the state, comment and
migration stores on the transaction's connection (`sqlitestore.TxConn`), so a
failed import rolls back all of them. In place, a setting or migration record
the database already holds is kept and listed, and the database files are
excluded from the source-changed check. Verified on a copy of `tickets/`: 6325
entities and 7784 relations, matching the files; a forced re-run fails on every
collision and leaves the database unchanged.

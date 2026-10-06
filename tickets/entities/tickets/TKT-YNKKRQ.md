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

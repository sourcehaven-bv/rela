---
id: DEC-R9M57Z
type: decision
title: SQLite projects may carry their config in rela.db, layered behind disk
context: A self-contained desktop app needs its schema, scripts and UI config in the same file as its data. CLAUDE.md said the metamodel is always read from disk, which made a single shippable file impossible.
consequences: Config readers take a loader instead of reading files directly. Disk wins over the database, so editing a project keeps working. A shipped rela.db can be inspected and exported with rela db dump. fs and postgres builds are unchanged.
date: "2026-10-03"
status: accepted
---

## Decision

On the sqlite build, a project's operator config may live in `rela.db` (the
`project_files` table), so one file is a shippable app. This reverses the rule
that the metamodel is always read from disk, for sqlite only.

Disk stays first. The database is a fallback layer behind it
(`config.NewLayered(rootfs.New(root), configsql)`): a project with both is one
being edited, and the file the operator just wrote must win.

## Rules that follow

- Every reader of operator files goes through the loader: schema and
`acl.yaml`, scripts, `custom/` and `apps/`, templates, `data-entry.yaml`.
- The disk layer is `internal/rootfs`, which keeps `os.Root` containment
per directory.
- `.rela/secrets.yaml` is never stored in the database.
- `rela db load` / `rela db dump` and the desktop File menu move config and
markdown data in and out.

---
id: TKT-NUMUJD
type: ticket
title: Changing an entity type's plural silently hides its entities
kind: chore
priority: medium
effort: s
status: backlog
---

## Description

Changing an entity type's `plural:` in schema.yaml (by hand, through Configure,
or on rela-server) without renaming the directory under `entities/` makes
fsstore skip the old directory on the next open. Every entity of that type
disappears from reads, and sequential ids can be issued again. Nothing warns the
operator.

`rela rename entity --plural` renames the directory but skips the directory name
check that TKT-C2YVX1 added to the schema loader, so a bad name leaves a
half-renamed project that does not load.

Found in the second code review of TKT-C2YVX1 (RR-JGIBVE, RR-60LCF1). TKT-C2YVX1
only makes a hot reload refuse the change while entities exist and tells the
operator to rename the directory.

## Fix direction

- On open (and on a Configure save that changes a plural), detect a declared
type whose directory is missing while an undeclared directory with entity files
of that type exists, and fail or warn with the rename to do; or offer a data
migration step that moves the directory.
- `rela rename entity --plural` validates the new name with the same check as
the loader before touching anything.

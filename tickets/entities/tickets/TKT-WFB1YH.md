---
id: TKT-WFB1YH
type: ticket
title: 'Boot a SQLite project from its database: schema and acl.yaml through the layered config loader'
kind: enhancement
priority: high
effort: m
status: ready
---

## Description

The sqlite recipe opens the database AFTER prepare() has already read
schema.yaml and acl.yaml from disk, so a project whose config lives only in
rela.db fails with "load metamodel". Open the connection first, build the
disk-first layered config.Loader over it, and have prepare() read the metamodel
(including includes and migration detection) and acl.yaml through that loader.
The same handle then goes to sqlitestore.New, so there is one connection per
project.

Also: project discovery and the schema hot-reload path must accept a project
that has only .rela/rela.db.

## Acceptance criteria

- A directory with only .rela/rela.db (config in project_files) boots rela-sqlite and rela-server-sqlite.
- Disk still wins when both exist.
- fs/memory/postgres builds unchanged.

---
id: TKT-LWOCW9
type: ticket
title: 'rela db dump / rela db load: export and import a project''s config (and data) to and from rela.db'
kind: enhancement
priority: high
effort: m
status: ready
---

## Description

configsql.Loader.Put and Paths exist for this but nothing calls them. Add
sqlite-build commands:

- rela db load [dir]: copy the project's config files into project_files; with --data, also import markdown entities and relations into an empty store.
- rela db dump <dir>: write project_files out as files; with --data, also write entities and relations as markdown.

Both are operator-shell, audited, raw-store writes like rela db migrate.

## Acceptance criteria

- dump then load round-trips byte-identically.
- A markdown project can be converted into a self-contained rela.db and back.

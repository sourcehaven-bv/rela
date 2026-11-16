---
id: TKT-P9QMEVI
type: ticket
title: Bound the remaining Lua read bindings
kind: enhancement
priority: medium
effort: m
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

## Description

TKT-FED6FMM left these `request-path` entries in
`internal/archguard/unpagedread_allowlist_test.go`:

- `internal/lua/runtime.go#Runtime.luaGetRelations` (`rela.get_relations`) and `internal/lua/elevation.go#elevatedGetRelations` (`admin.get_relations`): the script's from/type/to, possibly none, with no limit.
- `internal/lua/elevation.go#elevatedListEntities` (`admin.list_entities`): every entity of a type, with no limit.
- `internal/lua/markdown.go#mdEntityRefs.luaMdEntityRefs` (`rela.md.entity_refs`): reads every entity of the named types to build a replacement map; it could read only the ids referenced in the markdown.

`rela.list_entities` is already bounded by `maxReadLimit` (DEC-IYHLNF) and was
recategorized `bounded`. The others need the same `limit` option and ceiling,
which is a script API change (DEC-IYHLNF stage 2), and `md.entity_refs` needs an
id batch.

## Script API changes

- `rela.get_relations` and `admin.get_relations` take a `limit` option. The
default and the ceiling are `maxReadLimit` (2000). `limit = 0`, a fraction or a
non-number raises. Unknown keys are still ignored, for older scripts.
- `rela.get_relations` now raises on a store error instead of returning a
short list. Hidden edges do not count toward the limit.
- `admin.list_entities` takes `limit` with the same rules.
- `rela.md.entity_refs` takes `ids` or `content` (the ids a body mentions in
code spans). Both read only those ids in one batch. Without them it keeps the
old per-type read; past 2000 readable entities it raises unless the script sets
`limit`, which then cuts the map at that many.
- `rela.md.entity_refs` now rejects unknown option keys and malformed ids.
- `rela.get_relations` fails on an ACL gate fault instead of returning a
thinned list.
- A script that reads more than 2000 rows now gets the first 2000. Cursor
paging is the follow-up.

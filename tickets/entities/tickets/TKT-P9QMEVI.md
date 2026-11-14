---
id: TKT-P9QMEVI
type: ticket
title: Bound the remaining Lua read bindings
kind: enhancement
priority: medium
effort: m
status: backlog
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

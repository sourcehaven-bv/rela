---
id: TKT-Q02WIFH
type: ticket
title: Cursor paging for the Lua read bindings
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

TKT-P9QMEVI bounded `rela.get_relations`, `admin.get_relations`,
`admin.list_entities` and `rela.md.entity_refs` at `maxReadLimit` (2000). A
script that needs more rows gets the first 2000 and cannot continue. Add the
`cursor` option of DEC-IYHLNF stage 2 to the list bindings. The gated relation
page (`visibility.ScriptReader.ListRelationsPage`) already ends each full page
on a visible row, so its NextCursor never encodes a hidden key. The entity
cursors must keep that property.

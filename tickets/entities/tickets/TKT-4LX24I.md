---
id: TKT-4LX24I
type: ticket
title: Lua list_entities face option
kind: enhancement
priority: medium
effort: xs
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: 'rela.list_entities and admin.list_entities list each entity at the face the default world serves. Add a face option that lists the rows of one named face instead (GitHub #1761).'
---

## Description

Since #1753 (TKT-7IZHP0) `rela.list_entities`, `admin.list_entities`,
`rela.md.entity_refs` and `rela.search` read in the default world, and every row
carries `face`. That fixes the empty result #1761 reported. A script still
cannot list the rows of a face the world does not serve first, for example every
`concept` row when the world serves `vastgesteld`.

## Change

- `rela.list_entities(type, { face = "concept" })` lists the rows at that face (`store.AtFaces`). It composes with `filter` and `limit`. A face the type does not declare raises.
- `admin.list_entities(type, { face = ... })` takes the same option. Other keys raise.

A `world` option is left out: the runtime holds only the default world, and
resolving a named world would need the compiled worlds wired into `ReadDeps`.

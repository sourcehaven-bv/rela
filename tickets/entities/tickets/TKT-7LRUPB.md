---
id: TKT-7LRUPB
type: ticket
title: 'Lua rela.tagged_entities: find entities by version tag'
kind: enhancement
priority: medium
effort: m
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

## Description

Auditors ask "what changed since the last audit". Product Lua answers that with
version tags (`rela.tag_version`, `rela.version_by_tag`), but a script can only
look a tag up on an entity it already knows, so entities deleted since the
baseline cannot be found.

Add `rela.tagged_entities(name [, {type=}])`:

```lua
local rows = rela.tagged_entities("baseline/audit-2026", { type = "maatregel" })
-- each row: { id=, face=, type=, version=, deleted=true|false }
```

- Returns every entity (optionally of one type) that has a version tagged
`name`, including entities deleted since, flagged `deleted`.
- Gated like the other history reads: rows the principal may not read are
omitted; a deleted entity requires `history:read` as for the history of a
deleted entity (docs/acl-security.md).
- One store query per call, no per-row lookups. SQLite and PostgreSQL; other
backends raise the "version history is not supported" error.
- Docs in the Lua guide with a short "baseline and compare" example.

Source: G5 brief (`.ignored/gaps/G5-freeze-and-baselines.md`), approved by the
user.

## Out of scope

- Version tags on relations (backlog).
- Graph-wide `as_of` reads for views and exports (backlog).

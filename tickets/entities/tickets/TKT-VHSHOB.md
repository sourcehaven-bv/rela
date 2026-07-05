---
id: TKT-VHSHOB
type: ticket
title: CLI table output ignores display_property (uses literal title only)
kind: enhancement
priority: medium
effort: s
status: in-progress
---

## Problem

The CLI table output (`rela list`, `rela export`, `rela graph`, `rela delete`
confirmation) renders an entity's title via `entity.Title()`
(`internal/entity/entity.go:103`), which reads only the literal `title`
property. It never consults the metamodel's `DisplayTitle`, so it honors
**neither** the bare-name `display_property` (TKT-RT3Y3) **nor** the new
template form (TKT-NJTBQX). Any entity type whose display name isn't literally
`title` shows a blank TITLE column in `rela list`.

Verified on develop: a `persoon` type with `display_property: achternaam`
already shows blank — this is a long-standing gap, surfaced while testing the
template feature.

## Scope

Wire `metamodel.DisplayTitle` into the CLI output path so the table/graph/export
title columns match what the data-entry web app shows.

- `internal/output/output.go` `writeEntitiesTable` (line ~103) calls
`e.Title()`; it needs the metamodel to call `meta.DisplayTitle(e.ID, e.Type,
e.Properties)` instead.
- The `output.Writer` currently has no metamodel handle — thread one in (or pass
a resolved title per row from the CLI command, which already has `svc.Meta()`).
- Same for `export.go` (node titles), `graph.go` (node labels), `delete.go`
(confirmation prompt).

## Acceptance criteria

1. `rela list persoon` with `display_property: achternaam` shows the achternaam value, not blank.
2. `rela list persoon` with a template `display_property` shows the rendered template.
3. Entities with a literal `title` property are unchanged.
4. `rela export`/`graph` node titles/labels honor `display_property`.

## Out of scope

- Any change to `DisplayTitle` itself (done in TKT-NJTBQX / TKT-RT3Y3).

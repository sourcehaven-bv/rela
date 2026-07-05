---
id: TKT-VHSHOB
type: ticket
title: CLI table output ignores display_property (uses literal title only)
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

The CLI table output (`rela list`, `rela graph`, `rela delete` confirmation,
`rela show` detail) renders an entity's title via `entity.Title()`
(`internal/entity/entity.go:103`), which reads only the literal `title`
property. It never consults the metamodel's `DisplayTitle`, so it honors
**neither** the bare-name `display_property` (TKT-RT3Y3) **nor** the new
template form (TKT-NJTBQX). Any entity type whose display name isn't literally
`title` shows a blank TITLE column in `rela list`.

Verified on develop: a `persoon` type with `display_property: achternaam`
already shows blank — this is a long-standing gap, surfaced while testing the
template feature.

## Scope

Wire `metamodel.DisplayTitle` into the human-facing CLI output so title
columns/labels match what the data-entry web app shows.

- `internal/output/output.go`: a consumer-side `TitleResolver` interface + a
`Titles` field on `Writer`; `entityTitle` helper resolves via the metamodel when
set, else falls back to the literal `title` property. Used by the entity table
(`writeEntitiesTable`) and the detail view (`WriteEntity`).
- `internal/cli/kong.go`: sets `out.Titles = cliSvc.Meta()` once the project's
metamodel is loaded.
- `internal/cli/graph.go`: node labels via `DisplayTitle` (with an ID-fallback
guard so a titleless entity doesn't render `"ID\nID"`).
- `internal/cli/delete.go`: confirmation prompt via `DisplayTitle`.

## Acceptance criteria

1. `rela list persoon` with `display_property: achternaam` shows the achternaam value, not blank. **DONE — verified.**
2. `rela list persoon` with a template `display_property` shows the rendered template. **DONE — verified ("Jeroen Vloothuis").**
3. Entities with a literal `title` property are unchanged. **DONE — verified.**
4. `rela graph` node labels honor `display_property`. **DONE — verified (`label="ID\n<rendered>"`).**

## Deliberate exclusions (decided during code review)

- **`rela export` stays raw.** Export is a data-interchange format (JSON/CSV/YAML)
with `title,omitempty`; resolving via `DisplayTitle` would echo the entity ID as
the title for titleless entities, breaking the "absent title" signal that
machine consumers rely on. Export keeps `node.Title()` (raw property).
- **`rela trace`/`path` NOT covered here.** The tracer builds its own node
titles (`internal/tracer`, a metamodel-free pure reader) and doesn't route
through the writer's resolver. Fixing it cleanly means resolving at the output
boundary — tracked separately as **TKT-COZN2E**.

## Out of scope

- Any change to `DisplayTitle` itself (done in TKT-NJTBQX / TKT-RT3Y3).

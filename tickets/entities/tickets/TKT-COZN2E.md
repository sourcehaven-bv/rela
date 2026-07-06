---
id: TKT-COZN2E
type: ticket
title: rela trace/path output ignores display_property (tracer builds titles from literal title)
kind: enhancement
priority: low
effort: s
status: review
---

## Problem

`rela trace` / `rela path` render node titles that are built inside
`internal/tracer/tracer.go` via `e.Title()` (the literal `title` property) at
lines 98, 153, 192, 233, 259. So — like the CLI table before TKT-VHSHOB — trace
output shows a blank title for any entity type whose display name isn't
literally `title` (bare-name or template `display_property`).

Surfaced during the TKT-VHSHOB code review: that ticket wired `DisplayTitle`
into the `output.Writer` (list/show/detail) and into graph, but the tracer
builds its own `TraceNode.Title` / `PathStep.Title` and does not route through
the writer's resolver.

## Constraint

`tracer` is a **pure reader with no metamodel dependency** by design (see the
architecture rules in CLAUDE.md — "tracer is a pure reader"). So we should NOT
inject the metamodel into the tracer.

## Preferred approach

Resolve display titles at the **output boundary**, not in the tracer:
- Stop populating `TraceNode.Title` / `PathStep.Title` with `e.Title()` in the
tracer (leave the raw title, or carry `Type` + `Properties` on the node).
- Have `output.WriteTrace` / the trace command resolve each node's title via the
same `TitleResolver` the entity table uses (`entityTitle`), so table and trace
agree.

This makes the "table and trace paths agree" contract actually true and reuses
the TKT-VHSHOB resolver.

## Acceptance criteria

1. `rela trace <id>` and `rela path <a> <b>` show the display_property-resolved title (bare name or template), matching `rela list`.
2. Literal-`title` types unchanged.
3. The tracer package gains no metamodel dependency (`just arch-lint` stays green).

## Out of scope

- Changing `DisplayTitle` itself (done in TKT-NJTBQX / TKT-RT3Y3).

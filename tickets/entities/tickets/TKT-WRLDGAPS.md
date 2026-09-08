---
id: TKT-WRLDGAPS
type: ticket
title: Subsystems that read entities but cannot select a world (MCP, Lua, scheduler, CLI, transform)
kind: enhancement
priority: high
effort: l
status: backlog
---

FEAT-9CD2MX made the **data-entry HTTP API** world-aware. Every other surface
that reads entities still serves default faces only, silently.

Surveyed 2026-08-28 against `tkt-worlds-integration`. Search is excluded here —
it is TKT-9KZGJO, which now carries a decided design.

| Surface | Files touching the store | World-aware |
|---|---|---|
| **MCP** | 11 | **none** |
| **Lua** (`ReadDeps`) | 4 | none |
| **CLI** | — | 3 of 52 files mention a world |
| **transform** (export) | 1 | none |
| **attachment** | 2 | none |

Not affected — no store access: `calfeed`, `sync`, `importer`, `nextaction`.
`scheduler` reads only through `lua.WriteDeps`, so it inherits whatever Lua can
express; fixing Lua fixes the scheduler.

## Why this is one ticket, not five

The shape is identical everywhere: a read path that takes a `store.EntityQuery`
(or wraps one) but has no way for a caller to set `Query.World`. **The field
already exists** (`store.go:308`) — this is a plumbing and surface-API gap, not
a storage one.

## Ranked

**1. MCP — highest.** The largest unaddressed surface, and the one where being
wrong is most misleading: an agent asking an ISMS about a policy through
`show_entity` / `list_entities` / `trace_from` gets the DRAFT, with nothing in
the response saying so. A human at least chose a URL; an agent has no such
signal. Almost certainly plumbing rather than design.

**2. Lua.** `lua.ReadDeps` already passes `store.EntityQuery` through
(`deps.go:28`), so the plumbing is present and unreachable — a script cannot
name a world. Probably the cheapest of the five, and it closes the scheduler
with it.

**3. CLI.** `rela list` may honour a world; `analyze`, `trace`, `export` do not.
Inconsistent flags across sibling commands are their own trap.

**4. transform / export.** Export is documented as "downstream of an already-
authorized view, never a new capability" — so an export taken under a world
should render that world's faces. It renders default faces instead.

**5. attachment.** Needs a DESIGN answer first, unlike the others: are
attachments face-scoped at all? A draft and its published face may share a PDF
or may not, and that is a metamodel question, not wiring. Do not build until it
is answered.

## Do not

Do not add a per-subsystem notion of "which face". A world is already the answer
(see TKT-9KZGJO's decided design) — every one of these should take a
`store.WorldScope` and pass it down, not invent a parallel selector.

Related: [[TKT-9KZGJO]], [[FEAT-9CD2MX]].

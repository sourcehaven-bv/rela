---
id: FEAT-5IUVGX
type: feature
title: Format-agnostic view export via a transform registry
summary: Register named markdown->format transforms (pdf, docx...) once in the metamodel; every renderable surface (entity view, list view, Lua doc) gains those formats automatically as an ACL-gated 'Export as ...' action.
description: Register named markdown->format transforms (pdf, docx, ...) once in the metamodel; every markdown-producing surface (entity view, list view, Lua document) automatically gains those formats as an ACL-gated 'Export as ...' action. Transforms are argv command templates run via the security-reviewed internal/attachment/cmdrunner.go exec engine (no shell, {{in}}/{{out}} temp paths, output cap, timeout). Renderers (built-in entity, Lua override, Lua doc, list table) share one narrow Render(ctx)([]byte,error) interface producing markdown. Export is a property of an already-ACL-gated view, so it can never reach what the view could not show. CLI + data-entry exposure; not raw over MCP.
priority: medium
status: proposed
---

## Summary

rela can already produce markdown (Lua documents; entity storage serialization).
This feature adds a **transform registry** so any markdown-producing surface can
be exported to arbitrary formats (PDF, DOCX, ...) via external tools (pandoc,
weasyprint, typst, libreoffice), **without** hand-wiring each producer×format
pair.

Register a format **once** in the metamodel; it lights up everywhere a renderer
exists. The producer↔format relationship is a **cross product**, not an N×M
table.

## The three layers

1. **Transforms (registry).** A `transforms:` map in the metamodel. Each entry is a
named `from: markdown` → format byte shuttle: an argv `command` template with
`{{in}}`/`{{out}}` placeholders and a `produces` content-type. Transforms know
nothing about entities, Lua, or the web app — pure byte conversion.

2. **Renderers (produce markdown).** One narrow interface,
`Render(ctx) ([]byte, error)`, with three+ implementers:
   - **Entity** — built-in renderer (title→H1, props, resolved relations, body),
overridable per entity-type by a registered Lua render script.
   - **Lua document** — the existing `ExecuteDocument` stdout-capture path.
   - **List** — v1 built-in **table** (columns = the data-entry view's visible
columns), overridable via Lua for fancier output.

3. **Views (bind point).** Export is a property of an **already-ACL-gated view**, not
a standalone web capability. The entity view and list view expose an **"Export
as ▾"** menu whose items are exactly the registered `from: markdown` transforms.
Picking one runs the view's renderer against the **already-authorized set the
view loaded** and streams the transform output. Export can therefore never reach
anything the view could not already show.

## Safety model (load-bearing)

- Invocation is downstream of an authorization decision the view already made.
- A request may only choose a **registered format name** + operate on the view it
already loaded — never a command, flag, or path.
- Command templates are project-config trust (whoever authors `metamodel.yaml`),
identical to `schedules.yaml` Lua trust.
- **argv array, never `sh -c`**; `{{in}}`/`{{out}}` expand to rela-chosen temp paths.
Reuse the security-reviewed exec pattern in `internal/attachment/cmdrunner.go`
(bounded output, timeout, no shell).
- Fresh temp dir, timeout, output-size cap, clear "tool not on PATH" error.

## Scope decisions

- **Exposure:** CLI + data-entry (via ACL-gated views). Not raw over MCP.
- **List export:** table in v1 (Lua override for fancier); whole **filtered** set under
ACL, **capped** — past the cap, truncate and inject a visible "showing N of M
(truncated)" notice into the rendered markdown so it survives into the PDF.
- **Render override home:** `dataentryconfig` (next to `DetailView`/`List`), because
the codebase deliberately keeps presentation config out of the metamodel's
`EntityDef`. The `transforms:` registry itself stays in the metamodel
(domain-level, CLI sees it too).

## Prior art in-tree

- `FEAT-OT4361` (format-agnostic calendar/feed export) is the same shape: a pure model
  + pluggable serializers. Mirror that separation.
- `FEAT-KTZJIV` / `internal/attachment/cmdrunner.go` already provides the argv exec
engine (`{in}`/`{out}` templating, `maxBytes` cap, timeout, no shell). The
metamodel already has a `TransformStep{ Cmd []string }` shape to align with.

## Explicitly out of scope (v2)

- Format auto-chaining (md→html→pdf).
- Sectioned/per-entity list export (built-in); available via Lua override only.
- Async job model for slow converters (LaTeX, libreoffice).
- Transforms exposed raw over the MCP server.

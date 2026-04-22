---
id: FEAT-023
type: feature
title: Document Rendering in Data Entry Server
summary: Read-only markdown document panels attached to entity views, rendered by shell command or Lua script with caching and edit:// link rewriting.
description: Render documents from external commands in the data-entry UI, enabling live preview of composed documents built from rela entities.
priority: medium
status: implemented
---

# Document Rendering in Data Entry Server

## Status

**V1 (shipped)** — shell `command:` renderer producing markdown, rendered as
HTML panels in the data-entry UI with live-reload and `edit://`/`create://` link
rewriting. `DocumentConfig` in `data-entry.yaml`; rendering in
`internal/dataentry/document.go`.

**V2 (shipped via TKT-CGBVW)** — Lua `script:` renderer as an alternative to
`command:`. Exactly one of `{command, script}` must be set per document. Script
runs via `script.Engine.ExecuteDocument` with a writer runtime; captured stdout
is the rendered markdown. New context: `rela.mode == "document"`,
`rela.document.{id, entry_id}`. Lua renders bypass the disk cache (Lua's
in-process `rela.cache` is the caching story). The HTTP handler enforces
`entity_type:` before invoking any renderer.

## Overview

Render documents in the data-entry UI from either a shell command or a Lua
script. Composed documents (e.g. a category overview that walks related tickets)
render as HTML panels in the entity view with live-reload on SSE entity-change
events and clickable `edit://` / `create://` links that deep-link into the
data-entry forms.

## Follow-up work

Tracked separately:

- **TKT-E1FO1** — `rela.document.depends_on(id)` for SSE dependency tracking.
V1 live-reload fires only on entry entity changes; a Lua doc composed from many
entities relies on the refresh button when non-entry entities change.
- **TKT-CGPYW** — generalize `rela.mode` to other contexts (script / flow /
action / scheduled / validation) once a concrete need surfaces.

## Usage

```yaml
documents:
  ticket_summary:
    title: "Ticket Summary"
    entity_type: ticket
    command: "my-renderer {id}"   # shell render
    timeout: 30

  category_overview:
    title: "Category Overview"
    entity_type: category
    script: docs/category_report.lua  # Lua render
    timeout: 10
```

See `GUIDE-data-entry.md` (Documents section) for the full config schema, the
`edit://`/`create://` URL scheme, caching semantics, and the Lua document-mode
API.

## Original V1 design notes (historical)

The initial prototype was a hardcoded render endpoint invoking `mdcomp render`
with YAML context piped in. That shape shipped with a config-driven
`DocumentConfig` entry (`command:`) and goldmark conversion, plus `edit://` and
`create://` URL rewriting to support in-document navigation.

Success criteria (all met):

- Render a document at `/api/v1/_documents/{docName}/{entryID}`.
- Markdown → HTML via goldmark.
- `edit://` and `create://` links rewritten to form URLs with return param.
- Page reloads when entities change (via SSE).
- Errors handled gracefully (rendered as HTTP 500 with message).

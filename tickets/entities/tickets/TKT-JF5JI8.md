---
id: TKT-JF5JI8
type: ticket
title: Transform registry + view export (pdf/docx via external tools)
kind: enhancement
priority: medium
effort: l
tags:
    - needs-design
status: review
---

## Description

Add a **transform registry** so any markdown-producing surface can be exported
to arbitrary formats (PDF, DOCX, ...) via external tools. Register a `from:
markdown` transform once in the metamodel; every ACL-gated view (entity, list,
Lua document) gains it automatically as an **"Export as ▾"** action.

Implements `FEAT-5IUVGX`.

## What ships (v1)

1. **Metamodel `transforms:` map** — named entries, each `{ from, command (argv with
`{{in}}`/`{{out}}`), produces (content-type) }`. Parsed + validated; registered
in `checkUnknownKeys`.
2. **Exec engine** — reuse `internal/attachment/cmdrunner.go` (argv array, no shell,
`{in}`/`{out}` templating, `maxBytes` cap, timeout). No new subprocess code.
3. **`Renderer` interface** (`Render(ctx) ([]byte, error)` → markdown) with:
   - **Entity built-in renderer** (title→H1, props, resolved relations, body) — new.
   - **Entity Lua override** + **Lua document** — via existing
`script.ExecuteDocument` (stdout captured as markdown).
   - **List table renderer** — columns from `dataentryconfig.ListColumn`; new.
4. **Render override config** in `dataentryconfig` (next to `DetailView`/`List`), NOT
in metamodel `EntityDef`.
5. **data-entry**: `GET /api/transforms` (drives the menu) + export sub-resource on the
entity view (`visibleReader.getVisible`) and list view (`scopedSortedEntities`,
pre-pagination, capped). Streams bytes with `produces` content-type + download
name.
6. **CLI**: an export command wiring a renderer → transform.

## Safety (load-bearing)

- Export is a property of an already-ACL-gated view; never a standalone capability.
- Request picks only a **registered format name** + the view it already loaded — never
a command/flag/path.
- argv, no `sh -c`; temp dir; timeout; output cap; clear missing-tool error.
- Single-subject read-path Lua (rendering one requested entity/list) is the bounded
case CLAUDE.md permits — NOT a hot-path ACL predicate.

## List export specifics

- v1 = **table**; Lua override for fancier output.
- Scope = whole **filtered** set under ACL (not just the page), **capped at N**; past N
truncate and inject a visible "showing N of M (truncated)" line into the
markdown so it survives into the PDF.

## Out of scope (v2)

Format chaining (md→html→pdf); built-in sectioned list export; async jobs for
slow converters; transforms over MCP.

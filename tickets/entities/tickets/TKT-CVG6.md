---
id: TKT-CVG6
type: ticket
title: Document the full rela.md (markdown AST) Lua API
kind: docs
priority: low
effort: s
status: backlog
---

# Document the Full `rela.md` Markdown AST API

## Context

`docs/lua-scripting.md` currently documents only the task list portion of the
`rela.md` Lua module (added by TKT-RTH3). The rest of the module's public API
surface — `parse`, `render`, `headers`, `shift_headers`, `set_min_header_level`,
`extract_section`, `first_paragraph`, `concat`, node constructors (`heading`,
`paragraph`, `code_block`, `thematic_break`, `blockquote`, `list`), and
generation helpers (`link`, `ref`, `table`, `entity_table`) — is undocumented in
the user-facing docs.

This gap pre-existed TKT-RTH3 (which only added task list support to an
already-implemented but undocumented API). It was originally introduced in
TKT-XKRH which shipped the AST API without user docs.

## Scope

- Add a "Markdown AST" section under `## API Reference` in
`docs/lua-scripting.md` that documents the full `rela.md.*` surface
- Reference (or merge) the existing "Markdown AST: Task Lists" subsection
added by TKT-RTH3
- Include AST node shape reference (heading, paragraph, code_block, list,
blockquote, thematic_break, table, raw)
- Include realistic examples for the most common use cases (extract a
section, build a TOC, compose multiple entities into one document)

## Out of scope

- Adding new API surface (this is docs-only)
- Reorganizing the rest of `lua-scripting.md`

## Acceptance Criteria

1. `docs/lua-scripting.md` has a complete reference table for `rela.md.*`
2. Each function lists its parameters, return type, and a short example
3. AST node shapes are documented for each node type
4. The existing "Task Lists" subsection from TKT-RTH3 is preserved or
integrated cleanly
5. `markdownlint docs/lua-scripting.md` passes

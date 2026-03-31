---
id: TKT-XKRH
type: ticket
title: Add Markdown AST API to Lua scripting
kind: enhancement
priority: medium
effort: m
status: review
---

# Add Markdown AST API to Lua Scripting

Extend the Lua scripting environment with a Markdown AST API that enables
scripts to:
- Parse entity/relation content into a manipulable AST
- Apply transformations (header shifting, extraction, etc.)
- Compose documents by combining multiple markdown fragments

## Motivation

Currently Lua scripts can access entity content as raw strings. For document
generation use cases, scripts need to:
- Shift headers when including entities in larger documents
- Extract specific sections (e.g., for summaries)
- Build composite documents from multiple sources

## Proposed API

Inspired by mdcomp filters:

```lua
-- Parse markdown to AST
local ast = rela.md.parse(entity.content)

-- Transformations
ast = rela.md.shift_headers(ast, 1)  -- +1 level (# becomes ##)
ast = rela.md.extract_headers(ast, {min_level=1, max_level=2})

-- Render back to markdown
local output = rela.md.render(ast)

-- Utility functions
local toc = rela.md.headers(ast)  -- [{level=1, title="Intro"}, ...]
```

## Use Cases

1. **Document Generation**: Compose a requirements doc from multiple entities
2. **Report Generation**: Extract summaries and combine with headers
3. **Export Workflows**: Transform content for different output formats

---
id: TKT-2Z3E
type: ticket
title: Add GFM table parsing and serialization to Lua markdown AST
kind: enhancement
priority: medium
effort: m
status: done
---

# Add GFM Table Parsing and Serialization to Lua Markdown AST

## Problem

The `rela.md.parse()` function currently uses goldmark with default
configuration, which does not include the GFM (GitHub Flavored Markdown) table
extension. Markdown tables fall through to the `default` case in `nodeToLua()`
and are returned as `{type = "raw", content = "..."}` — losing all structure.

## Scope

- Enable goldmark GFM table extension in `internal/lua/markdown.go`
- Map `*east.Table`, `*east.TableRow`, `*east.TableCell` to structured Lua tables in `nodeToLua()`
- Support rendering table AST nodes back to markdown via `rela.md.render()`
- Ensure existing `rela.md.table()` generation function continues to work
- Column-aligned padding in rendered tables with alignment-aware justification
- Display-width-aware padding using runewidth for CJK/emoji support
- Defensive width guard in renderSeparator to prevent panics

## Lua API

```lua
local ast = rela.md.parse(content)
-- Table nodes:
-- {
--   type = "table",
--   alignments = {"left", "center", "right"},
--   header = {"Name", "Value", "Notes"},
--   rows = {
--     {"foo", "42", "first"},
--     {"bar", "99", "second"},
--   }
-- }

-- Round-trip: parse → render preserves table content with aligned columns
local output = rela.md.render(ast)
```

## Follows Up

This extends the work from TKT-XKRH which added the initial markdown AST API but
did not include table support.

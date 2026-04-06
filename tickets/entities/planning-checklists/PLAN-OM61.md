---
id: PLAN-OM61
type: planning-checklist
title: 'Planning: Add GFM table parsing and serialization to Lua markdown AST'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Enable goldmark GFM table extension in `internal/lua/markdown.go`
- Parse GFM tables into structured Lua table nodes in `nodeToLua()`
- Render table AST nodes back to markdown in `renderNode()`
- Round-trip stability (parse → render → parse → render)

OUT of scope:
- Changes to `rela.md.table()` / `rela.md.entity_table()` generation functions (they work fine)
- Changes to `internal/markdown/format.go` or `normalize.go`
- HTML rendering changes (data entry helpers already use GFM)
- Column alignment support in `rela.md.table()` generator

**Acceptance Criteria:**

1. `rela.md.parse()` returns structured table nodes with type="table", header cells, data rows, and column alignments
2. `rela.md.render()` converts table AST nodes back to valid GFM markdown tables
3. Round-trip: parse → render → parse produces equivalent AST
4. Existing `rela.md.table()` and `rela.md.entity_table()` functions still work
5. Mixed content (headings + tables + paragraphs) parses correctly
6. Tables with alignment markers (`:---`, `:---:`, `---:`) preserve alignment info

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- goldmark v1.7.16 (already a dependency) has built-in GFM extension including table support
- `extension.GFM` is already used in `internal/dataentry/helpers.go:229-233` for HTML rendering
- GFM table AST types in `github.com/yuin/goldmark/extension/ast`: `Table`, `TableHeader`, `TableRow`, `TableCell`, `Alignment`
- Previous ticket TKT-XKRH implemented the markdown AST API but explicitly skipped tables
- The `nodeToLua()` pattern at `markdown.go:493-536` provides a clear template for adding table cases

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Enable GFM extension** (`markdown.go:76`): Change `goldmark.New()` to `goldmark.New(goldmark.WithExtensions(extension.NewTable()))` — use just the table extension, not full GFM, to avoid unintended side effects from strikethrough/footnotes etc.

2. **Add node type constant**: `nodeTypeTable = "table"`

3. **Add `nodeToLua()` case** for `*east.Table`: Build a Lua table with:
   - `type = "table"`
   - `alignments` = Lua table of alignment strings ("left", "center", "right", "none")
   - `header` = flat Lua string array (e.g. `{"Name", "Value"}`)
   - `rows` = Lua table of flat string arrays (e.g. `{{"foo", "42"}, {"bar", "99"}}`)

4. **Add `renderNode()` case** for table type: Generate GFM markdown with pipe-delimited rows and alignment separators.

5. **Helper functions**: `extractTableData()` to walk Table children and extract structure, `renderTable()` to format back to markdown.

**Design decision (from RR-V409):** Use flat string arrays for cells, not nested
`{text=...}` tables. This is consistent with how headings/paragraphs use flat
text fields and avoids unnecessary nesting.

**Revised Lua API:**
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
```

**Alternatives considered:**
- Use full `extension.GFM`: Rejected — would change parsing behavior for strikethrough, autolinks, etc. which could break existing scripts
- Parse tables in Lua with string splitting: Rejected — fragile, doesn't handle edge cases
- Only use `extension.NewTable()`: Chosen — minimal change, only adds table support
- Nested `{text=...}` cell tables: Rejected (RR-V409) — inconsistent with existing flat-string patterns

**Files to modify:**
- `internal/lua/markdown.go` — parser init, nodeToLua, renderNode, new helpers
- `internal/lua/markdown_test.go` — tests for parse, render, round-trip

**Dependencies:**
- `github.com/yuin/goldmark/extension` (already available, just not imported in this file)
- `github.com/yuin/goldmark/extension/ast` (GFM AST types)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Input: markdown strings from entity content (already in Lua sandbox)
- No new external input sources — goldmark parser is already trusted
- Cell text extraction uses same `extractText()` as other node types

**Security-Sensitive Operations:**
- None — this is pure in-memory parsing/rendering within the existing Lua sandbox

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Criterion | Test |
|-----------|------|
| Structured table nodes | Parse simple table, check type/header/rows/alignments |
| Render back to markdown | Build table AST in Lua, render, verify output |
| Round-trip stability | Parse → render → parse → render, compare outputs |
| Existing functions work | Run rela.md.table() and entity_table() after change |
| Mixed content | Parse doc with heading + table + paragraph, verify all nodes |
| Alignment preservation | Parse table with `:---:` markers, check alignment values |

**Edge Cases:**
- Empty table (header only, no data rows)
- Single-column table
- Table cells with inline formatting (bold, code, links)
- Pipe escaping behavior (verify goldmark's actual handling, per RR-EUC7)
- Table immediately following a heading (no blank line)

**Negative Tests:**
- Render a table node with missing header field → graceful handling
- Render a table node with empty rows → valid output

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Low risk: GFM table extension changes parsing of non-table content → Mitigated by using `extension.NewTable()` only
- Low risk: Inline formatting in cells loses detail → Accept: extract as plain text, same as headings/paragraphs

**Effort:** S (small) — the infrastructure is all there, this is adding one more
node type case

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: extends existing rela.md API, no new commands)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no docs changes needed)

**Documentation Impact:**
- N/A - Extends existing Lua API, no new user-facing commands or config

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-V409 (minor, addressed), RR-EUC7 (nit, addressed)

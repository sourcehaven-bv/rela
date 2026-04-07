---
id: PLAN-KUJE
type: planning-checklist
title: 'Planning: Add task list (checkbox) support to Lua markdown AST'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- Enable goldmark `TaskList` AND `Strikethrough` extensions in `luaMdParse`
- Extend list item representation: items can be strings (backward compat) or
tables with `text`, `checked`, and `task` fields
- Update `renderList` to detect task items (`task == true`) and serialize
`- [x]`/`- [ ]`
- Renderer always emits checkbox syntax for items with `task=true`, regardless
of position in a mixed list
- **Preserve strikethrough markers in extracted text**: update
`extractInlineText` to wrap `*east.Strikethrough` nodes with literal `~~...~~`
markers so the checklist "skip" semantics survive round-trips
- Backward compatibility: existing scripts using plain string items work unchanged

Out of scope:
- Nested task lists
- Other inline formatting preservation: bold, italic, links — see Known Limitations
- Multi-paragraph or block-content list items — see Known Limitations
- New helper functions beyond parse/render/construct

**Known Limitations (documented for users):**

1. **Inline formatting other than strikethrough is lost on round-trip**: Items
with `**bold**`, `*italic*`, or `[links](url)` lose that formatting when
extracted to text. Strikethrough IS preserved (see Scope) because the checklist
validation layer uses it as a "skip" marker.
2. **Multi-block items lost**: Items with nested lists, code blocks, or
multiple paragraphs only capture the first text block. Matches existing
list-item behavior.
3. **Mixed list round-trip stability**: A list mixing task and plain items may
not be stable across multiple parse/render cycles because goldmark only parses
items as tasks when each item carries a checkbox. The renderer always emits
checkboxes for `task=true` items, but re-parsing may classify items differently
than the original AST.

**Acceptance Criteria:**

1. `rela.md.parse("- [x] done\n- [ ] todo\n")` produces a list node whose
items are tables with `task=true`, `checked=true/false`, and `text` set
2. `rela.md.render()` serializes items with `task=true` as `- [x] text` or
`- [ ] text` (and `1. [x]`/`1. [ ]` for ordered lists)
3. `rela.md.list({{task=true, checked=true, text="x"}})` round-trips through
render to `- [x] x\n`
4. Existing string list tests pass unchanged (backward compatibility)
5. `task=false` or missing `task` field renders as a plain item (no checkbox)
6. Renderer treats missing/non-string `text` as empty string (no crash)
7. **Strikethrough markers survive round-trips**: parsing
`- [x] ~~done~~` produces a task item whose `text` field is `~~done~~`, and
rendering it back yields `- [x] ~~done~~`

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- goldmark `extension.TaskList` exposes `extast.TaskCheckBox` nodes. The
`TaskCheckBox` is an **inline node**, child of the `TextBlock` inside a
`ListItem` (NOT a direct child of `ListItem`).
- goldmark `extension.Strikethrough` exposes `*east.Strikethrough` inline
nodes. Used in `internal/markdown/content.go:124` for checklist validation.
- Reference pattern: `internal/markdown/content.go:114` uses
`ExtractChecklistItems()` with a walker that finds `TaskCheckBox` and detects
`*extast.Strikethrough` siblings to mark items as skipped.
- `internal/lua/markdown.go:563` `extractInlineText()` currently recurses into
emphasis/strikethrough nodes but emits only raw text, dropping the markers.
- `internal/lua/markdown.go:606` `extractListItems()` currently only walks
`TextBlock`/`Paragraph` direct children of `ListItem` and flattens to text.
- GFM table support pattern (TKT-2Z3E) is the closest analogue.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Parser (`luaMdParse`)**: Add `extension.TaskList` AND
`extension.Strikethrough` to goldmark options alongside `extension.NewTable()`.
Two-line change.

2. **Inline text extraction (`extractInlineText`)**: Add a case for
`*east.Strikethrough`:
   ```go
   case *east.Strikethrough:
       sb.WriteString("~~")
       for child := n.FirstChild(); child != nil; child = child.NextSibling() {
           r.extractInlineText(sb, child, source)
       }
       sb.WriteString("~~")
   ```
This preserves the `~~` markers around the inner text. Other inline node types
(emphasis, links) keep current behavior (markers dropped).

3. **AST extraction (`extractListItems`)**: For each ListItem:
   - Walk into the first `TextBlock`/`Paragraph` child
   - Inspect its first inline child: if it's a `*extast.TaskCheckBox`, the
item is a task item — emit a Lua table: `{text = "...", task = true, checked =
bool}`
   - Otherwise, emit as a plain string (current behavior)
   - Use existing `extractText` (which calls `extractInlineText`) for the
text content. The `TaskCheckBox` inline node has no children that
`extractInlineText` would emit (it's a leaf), so it naturally produces no text —
but verify and add an explicit skip case if needed.

4. **Renderer (`renderList`)**: Loop over items. For each item:
   - If item is `*lua.LTable` AND `task` field is truthy (`task == true`):
emit `- [x] text` or `- [ ] text` (or `N. [x]`/`N. [ ]` for ordered)
   - If item is `*lua.LTable` without truthy `task`: emit text as plain item
(read `text` field; default to empty string if missing/non-string)
   - If item is `lua.LString`: emit as plain item (current behavior)

5. **Constructor (`luaMdList`)**: No structural change. Already passes through
tables. The renderer is responsible for interpreting fields.

**Alternatives considered:**

- **Separate `task_list` node type**: Rejected. Goldmark models task lists as
regular lists with checkbox children; keeping the same node type avoids doubling
the renderer surface and matches the AST natively.
- **Always represent items as tables (drop string form)**: Rejected. Would
break backward compatibility with existing scripts.
- **Add a `skipped` boolean field instead of preserving `~~` in text**:
Rejected. Would require teaching the renderer about a new field, and the
text-with-markers approach is consistent with how the checklist validation layer
already detects skips. It also means the rendered output is immediately
re-parseable without special handling.
- **Store inline AST for items (preserving all formatting)**: Rejected as out
of scope. Would be a much larger change touching the entire inline rendering
pipeline.

**Files to modify:**

- `internal/lua/markdown.go` — parser options, `extractInlineText`,
`extractListItems`, `renderList`
- `internal/lua/markdown_test.go` — new test cases

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Markdown content strings to `rela.md.parse()` — already sandboxed by goldmark
- Lua tables passed to `rela.md.list()` — renderer defensively handles missing
fields (empty string for missing text); type assertions return zero values for
wrong types (no panics)

**Security-Sensitive Operations:**

- None. Pure parse/render extension with no new I/O, file access, or
capability changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| # | Test | Verifies |
|---|------|----------|
| 1 | Parse `- [x] done\n- [ ] todo\n`, inspect AST | AC1 |
| 2 | Parse, then render, expect identical output | AC1+AC2 round-trip |
| 3 | Construct task list via `rela.md.list({{task=true, checked=true, text="x"}})` and render | AC3 |
| 4 | Existing string-only list tests still pass | AC4 |
| 5 | Render `{task=false, text="x"}` → plain item | AC5 |
| 6 | Render `{text="x"}` (no task field) → plain item | AC5 |
| 7 | Render `{task=true, text=nil}` → `- [ ] ` (empty text) | AC6 |
| 8 | Ordered task list: parse `1. [x] done\n`, render | Ordered task syntax |
| 9 | Mixed list: empirically verify what `parse("- [x] task\n- plain\n")` produces, then render with task=true items always emitting checkbox | Documents real behavior |
| 10 | Multi-block item (item with nested code block): parse without crash, document lost content | Limitation #2 |
| 11 | **Strikethrough preserved**: parse `- [x] ~~done~~`, verify text=`~~done~~`, render → `- [x] ~~done~~\n` | AC7 |
| 12 | **Strikethrough round-trip stability**: re-parse rendered output, verify identical | AC7 |
| 13 | **Bold/italic dropped (limitation #1)**: parse `- [x] **bold**`, verify markers dropped, document behavior | Limitation #1 |

**Edge Cases:**

- Empty task text: `- [x]` (just checkbox, no text) — render as `- [x] `
- Single-item task list — works
- All-task list — most common case, must be stable
- Unicode text in task items
- Strikethrough mid-text: `- [x] foo ~~bar~~ baz`
- Multiple strikethrough segments in one item
- Task item with very long text (no width limit changes needed)

**Negative Tests:**

- `rela.md.list({{task=true}})` (no text) → renders as `- [ ] ` without crash
- `rela.md.list({{task="yes"}})` (non-bool task) → treated as non-task (only
`task == true` is truthy in our check)

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Backward compatibility break**: Low. String items preserved as-is in both
parse output and render input. Existing tests gate this. NOTE:
`extractInlineText` change affects ALL inline-text extraction (paragraphs,
headings, etc.) — strikethrough markers will now appear there too. This could be
considered a fix or a behavior change. Add tests for paragraph with
strikethrough to lock in the new behavior.
- **goldmark TaskCheckBox detection**: Low. Pattern exists in
`internal/markdown/content.go`. We use a simpler detection (first inline child
of TextBlock) since we control the walk.
- **Mixed list behavior surprises**: Medium → Mitigated by test #9 which
empirically verifies and locks in goldmark's actual behavior, plus documented
limitation #3.

Effort: **S** (small, ~60 lines of code + tests)

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: Lua API extension, internal change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: S-sized)

**Documentation Impact:**

- [x] ~~N/A - Internal change, no user-facing docs needed~~

Note: The known limitations are documented in the plan and in code comments, not
in user-facing docs (the Lua API surface is undocumented externally).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-AT2I (significant): Strikethrough markers stripped — addressed by enabling Strikethrough extension AND wrapping with `~~...~~` in extractInlineText (AC7 + tests #11-12)
- RR-1KSJ (significant): Multi-block list items not supported — addressed via documented limitation #2 + test #10
- RR-5ZCF (significant): Mixed list goldmark behavior unverified — addressed via test #9 (empirical verification)
- RR-GQJT (significant): Renderer must produce parseable output — addressed via "always emit checkbox for task=true" policy
- RR-872O (minor): task=false semantics — addressed via AC5 + truthy check policy
- RR-340I (minor): Constructor field validation — addressed via renderer defensive handling (AC6)
- RR-YJ0U (nit): TaskCheckBox is inline child — addressed via documented walk pattern in approach

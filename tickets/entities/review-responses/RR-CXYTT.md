---
id: RR-CXYTT
type: review-response
title: Soft/hard breaks are flags on goldmark Text, not separate AST nodes
finding: Plan lists soft_break/hard_break as inline kinds but goldmark represents them as flags on *ast.Text (SoftLineBreak()/HardLineBreak()), not separate nodes. extractInlines must emit a synthetic break inline AFTER a text node whose flag is set, OR keep the flag on the text node. Pick the synthetic-after approach — it matches Pandoc/mdast and keeps the renderer simple.
severity: significant
resolution: extractInlines emits synthetic {type='soft_break'}/{type='hard_break'} after a Text node whose flag is set. Renderer emits '\n' for soft_break, '  \n' (CommonMark) for hard_break. Pinned in AC5.
status: addressed
---

# Finding

`*ast.Text` has `SoftLineBreak()`/`HardLineBreak()` flag accessors
(`/goldmark/ast/inline.go:83-89`). Soft and hard breaks are not distinct AST
nodes. The plan's inline-kinds list (`soft_break`, `hard_break`) is correct as a
Lua-side abstraction but doesn't say how `extractInlines` produces them.

# Resolution

In `extractInlines`, when emitting a `*ast.Text` node, also emit a synthetic
`{type="soft_break"}` or `{type="hard_break"}` inline *after* the text node if
its flag is set. This matches Pandoc and mdast and keeps the renderer logic
linear.

The renderer for a soft break emits `\n` (or a single space if we want
soft-line-break collapsing — pin the choice in tests). Hard break emits `  \n`
(two trailing spaces) per CommonMark.

Pin in tests:

- Paragraph with hard break inside renders to `foo  \nbar`.
- Paragraph with soft break renders to `foo\nbar` (or `foo bar`,
depending on choice).

---
id: TKT-LXYHQ
type: ticket
title: Resolve entity-ID references to titled links in Lua markdown output
kind: enhancement
priority: medium
effort: m
status: done
---

# Resolve Entity-ID References to Titled Links in Lua Markdown Output

## Problem

When Lua scripts compose documents from entity content, the body text often
contains references to other entities by raw ID — e.g. "...as discussed in
TKT-123 the approach is..." or "see PRE-007". In the source markdown those IDs
are plain text. When the generated document is rendered, the reader gets an
opaque ID with no link and no context (no title).

This is a natural pattern in rela: people refer to entities by ID inside body
text the way one cites tickets in a code comment. We want script-produced
documents to surface those references as proper links titled with each target's
title, so the output reads naturally and stays navigable.

## Proposal

Add support at the `rela.md` (markdown AST) Lua level to resolve entity-ID
references inside text/inline nodes and replace them with markdown links whose
text is the target entity's title.

Likely shape (subject to planning):

```lua
local ast = rela.md.parse(entity.content)
ast = rela.md.resolve_refs(ast, {
  -- optional: pattern, link-builder, fallback behavior, etc.
})
local out = rela.md.render(ast)
```

Where `resolve_refs`:

- Walks text/inline nodes only (not code spans, not code blocks, not link
text/URL — those should be left alone).
- Detects tokens that match the project's entity-ID shape (the metamodel
defines ID patterns per type).
- Looks each one up; if it resolves, replaces the token with a link whose
text is the target's title and whose URL is configurable (script-supplied
builder, defaulting to something sensible).
- Leaves unresolved tokens as plain text (and optionally reports them).

## Out of scope

- Live "backlinks" panel / hover previews (covered by `IDEA-011`).
- Generic markdown link-checking.
- Any change to how entities themselves are stored.

## Acceptance Criteria

1. A new `rela.md` Lua helper resolves entity-ID tokens in inline/text nodes
to markdown links titled with the target entity's title.
2. Code spans, code blocks, existing link text, and existing link URLs are
untouched.
3. Unresolved IDs are left as plain text and surfaced to the script (return
value or callback) so callers can warn.
4. The link URL is script-configurable (builder function or template).
5. Round-tripping a document with no entity references is a no-op.
6. Documented under the `rela.md` API reference (extends `TKT-CVG6`'s scope or
a follow-up doc).
7. Unit tests cover: simple replacement, multiple refs in one paragraph,
refs adjacent to punctuation, refs inside code spans (must NOT be replaced),
refs inside link text (must NOT be replaced), unresolved IDs.

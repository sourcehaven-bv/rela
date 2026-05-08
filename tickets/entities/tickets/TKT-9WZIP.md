---
id: TKT-9WZIP
type: ticket
title: 'Restructure rela.md AST: preserve inline structure (text → inlines)'
kind: refactor
priority: medium
effort: l
status: done
---

# Restructure rela.md AST to Preserve Inline Structure

## Problem

`rela.md.parse` returns a "block-level AST": each block node (paragraph,
heading, blockquote, list-item, table-cell) carries a flat `text` string where
the goldmark parser's inline structure has been collapsed at extraction time
(`extractInlineText` in `internal/lua/markdown.go`). Some inline markers are
re-emitted as literal characters (`` `code spans` ``, `~~strike~~     `), some
are dropped entirely (link wrappers, emphasis, autolinks, raw HTML, image alt
text).

This forces every consumer that needs to *recognize* an inline kind (e.g. "skip
code spans" or "preserve link text/URL") to re-scan the flat string and
reconstruct the structure. `rela.md.resolve_refs     ` (TKT-LXYHQ) had to add
run-length backtick matching and Unicode-aware boundary checks just to avoid
linking inside code spans — a class of fragility that does not exist in goldmark
itself, only in our flattened representation.

It also bakes irreversible information loss into parse:

- `[See TKT-1](https://example.com)     ` parses to plain text
`See TKT-1     ` — the URL is lost. `resolve_refs     ` then re-links it with a
different URL than the author wrote.
- Raw HTML in headings/paragraphs is silently stripped.
- Image alt text and titles are gone.

## Proposal

Replace the `text     ` field on block nodes with a structured `inlines     `
array. Each inline is a small Lua table identifying its kind and its
content/attributes:

```lua
node = {
  type = "paragraph",
  inlines = {
    { type = "text", text = "see " },
    { type = "code_span", text = "TKT-1" },
    { type = "text", text = " or " },
    { type = "link", url = "/x", inlines = { { type = "text", text = "this" } } },
    { type = "text", text = "." },
  },
}
```

Move the existing flattening logic (the "preserve backticks, preserve `~~     `,
drop emphasis, drop link wrappers" policy) **into the renderer**.
`renderParagraph     ` etc. walk `inlines     `, applying the same policy. End
result is identical bytes for the existing flatten-everything path, but with the
structure available to consumers that need it.

`resolve_refs     ` (TKT-LXYHQ) becomes much simpler: walk `inlines     `, skip
`code_span     ` and `raw_html     ` nodes, rewrite only `text     ` nodes'
`text ` field. The run-length backtick scanner and Unicode-boundary checks that
exist solely to compensate for flat strings can be deleted in a follow-up.

## Bundled fixes (in scope for this refactor)

These fall out naturally from preserving structure and are conceptually part of
the same change:

- **Link preservation.** Source links retain `url     ` and inline children
(the link text). Re-rendering produces `[text](url)     `.
- **Raw HTML preservation.** Inline raw HTML is preserved as a
`raw_html     ` inline node and re-emitted verbatim by the renderer.
- **Image preservation.** Images become `image     ` inlines with `url     `,
`alt     `, and optional `title     `.
- **Autolink preservation.** Autolinks become `autolink     ` inlines.

## Migration

To minimize breakage:

- Block constructors (`paragraph(s)     `, `heading(level, s)     `,
`blockquote(s)     `, `list(items)     `, table cells) accept either a string
(auto-wrapped to `{{type="text", text=s}}     `) or an `inlines     ` table.
- Existing in-tree scripts and tests that read `node.text     ` directly
must migrate to either `node.inlines     ` or a new flat-text view (TBD in
planning: provide `flatten(inlines) → string     ` helper, or a computed `text `
view).
- The `text     ` field itself is removed (no compat shim) — keeping it
alive double-stores the same data and invites scripts to drift.

## Out of scope

- Refactoring `resolve_refs     ` to use the new structure. (Follow-up
ticket once this lands; this ticket only enables it.)
- Frontend / data-entry markdown rendering changes.
- New inline kinds beyond what goldmark already produces.

## Acceptance Criteria

1. `rela.md.parse     ` emits block nodes with an `inlines     ` array; no
`text     ` field on paragraph/heading/blockquote/list-item/table-cell.
2. `rela.md.render     ` round-trips parse output to byte-equivalent
markdown for: plain paragraphs, headings, blockquotes, plain and task lists, GFM
tables, fenced code blocks, raw HTML blocks, thematic breaks.
3. Inline kinds preserved: `text     `, `code_span     `, `link     `, `image     `,
`autolink     `, `raw_html     `, `emphasis     `, `strong     `, `strikethrough
`, `soft_break    `, `hard_break     `.
4. **Link round-trip.** Source `[See TKT-1](https://example.com)     `
parses to a `link     ` inline and renders back to identical syntax.
5. **Raw HTML round-trip.** Source `<a name="x">     ` in a heading is
preserved.
6. Block constructors accept strings (auto-wrap) and inlines tables.
7. Helpers that previously consumed `text     ` (`shift_headers     `,
`extract_section     `, `headers     `, `first_paragraph     `, `concat     `,
`entity_table   `) work against the new structure with no API regression visible
to existing scripts that don't read inline structure directly.
8. All existing tests pass (after migration). All existing in-repo
Lua scripts (in `scripts/     `, `automation/     `, etc.) continue to work
after migration.
9. The existing `extractInlineText     ` policy (drop emphasis, preserve
`~~     ` and backticks) survives as the default `flatten(inlines)     `
behavior, so consumers that just want the old flat string can opt in via one
helper call.

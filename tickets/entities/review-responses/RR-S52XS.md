---
id: RR-S52XS
type: review-response
title: Inline constructors not committed to in plan; scripts will write {type='text'} literals
finding: Plan mentions exposing rela.md.text(s) 'for the most common case' but doesn't commit. If scripts ever build inlines (custom format callbacks, document composition), they need at minimum text/code_span/link constructors. Otherwise the API is awkward.
severity: minor
resolution: 'Inline constructors committed: rela.md.text(s), rela.md.code_span(s), rela.md.link_inline(text_or_inlines, url, title?), rela.md.raw_html(s). The new link constructor uses a different name (link_inline) to avoid collision with the existing string-returning rela.md.link. Pinned in AC10.'
status: addressed
---

# Finding

Plan: "Add Lua-exposed inline node constructors (or a single `rela.md.text(s)`
builder) for the most common case so scripts don't have to write `{type="text",
text=s}` by hand."

That's an "or", not a commitment. With nothing exposed, scripts that build
inlines write Lua literals like `{type="text", text=s}`. Workable, but ugly
enough that people will work around it (build their own helpers, copy our
internal constants).

# Resolution

Commit to a minimal set of inline constructors at the same level as the existing
block constructors (`paragraph`, `heading`, etc.):

- `rela.md.text(s)` → `{type="text", text=s}`
- `rela.md.code_span(s)` → `{type="code_span", text=s}`
- `rela.md.link(text_or_inlines, url, title?)` —
**note name collision** with the existing `rela.md.link(text, url) → string`
helper. The string-returning link helper has been around since the original
`rela.md` API; we shouldn't break it. Pick a different name
(`rela.md.link_inline`) or namespace under `rela.md.inline.*`.
- `rela.md.raw_html(s)` → `{type="raw_html", text=s}`

Skip `emphasis`/`strong`/`autolink`/`image` constructors for now; add if real
users ask.

Document in the `rela.md` reference.

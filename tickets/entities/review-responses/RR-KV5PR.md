---
id: RR-KV5PR
type: review-response
title: code_block and raw block nodes keep 'content' — spell out scope explicitly
finding: Plan addresses paragraph/heading/blockquote/list-item/table-cell. code_block and raw block nodes correctly stay with 'content' string fields (they're not inline-bearing). Plan should say so explicitly to prevent over-eager 'fix all the things' impulses.
severity: nit
resolution: 'Decisions section explicitly states: code_block and block raw nodes retain content: string fields. They are not inline-bearing; bodies are literal text emitted verbatim.'
status: addressed
---

# Finding

Plan changes `text`/`content` to `inlines` on
paragraph/heading/blockquote/list-item/table-cell. `code_block` and the
catch-all `raw` block nodes (not the *inline* `raw_html`) keep their `content`
string fields. That's correct — they're not inline-bearing — but the plan
doesn't say so, leaving room for an over-eager implementer to migrate them too
and break the renderer.

# Resolution

Add to "Decisions" or "Out of scope":

> `code_block` and the block-level `raw` nodes retain their
> `content: string` fields. They are not inline-bearing; their
> bodies are literal text segments emitted verbatim. No change to
> their shape.

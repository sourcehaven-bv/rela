---
id: RR-KBKFH
type: review-response
title: 'flattenInlines policy is internally contradictory: drops link wrappers AND emits link syntax'
finding: Plan says flattenInlines is 'the same policy as today's extractInlineText' (drop link wrappers — only text kept) AND 'emits [text](url) for links' (preserve link syntax). Pick one. The renderer needs link syntax preservation; helpers like first_paragraph/headers need the legacy text-only policy. Two functions, not one.
severity: significant
resolution: Split into two helpers. renderInlines(inlines)→string preserves link syntax, autolinks, raw HTML, images. flattenInlines(inlines)→string applies the legacy policy (drop link wrappers and emphasis, preserve `~~` and backticks). Pinned in AC11 and AC12.
status: addressed
---

# Finding

Plan paragraph A: "Renderer calls `flattenInlines` which applies the existing
policy: drop emphasis/strong wrappers (just inline children), drop link wrappers
(just inline children)..."

Plan paragraph B: "`flattenInlines` ... emits `[text](url)` for links."

Those contradict. The renderer wants link syntax preserved (so parse→render
round-trips). Helpers like `first_paragraph` and `headers` want the existing
text-only flattening (so script behavior is unchanged).

# Resolution

Two functions:

- `renderInlines(inlines) → string` — preserves link syntax,
raw HTML, autolinks. Used by block renderers (`renderParagraph`,
`renderHeading`, `renderBlockquote`, `renderListItemTable`, table cells if going
inline).
- `flattenInlines(inlines) → string` — applies the legacy
text-extraction policy (drop link wrappers, drop emphasis, preserve `~~` and
backticks). Used by `headers`, `first_paragraph`, the public `rela.md.flatten`,
and any helper that wants "what the user reads".

The two share a recursive walker but differ in per-kind emission. ~50 lines per
function, much overlap.

Pin in tests:

- Render of `[See TKT-1](https://x)` paragraph → identical
bytes.
- `flatten` of same paragraph → `See TKT-1` (no URL).
- `headers` of `# A [link](url) B` → `title = "A link B"`.
- `first_paragraph` of same → `A link B`.

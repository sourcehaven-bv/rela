---
id: RR-2N3O4
type: review-response
title: Image alt as flat string forfeits the structure-preservation thesis
finding: Plan stores image alt as a flattened string. Goldmark allows formatting inside alt text. Flattening at parse loses it. Rare but inconsistent with link/raw-HTML preservation. Either store alt as inlines or document why.
severity: minor
resolution: 'image inline shape: {type=''image'', url, title?, alt_inlines={...}}. extractInlines recurses into alt children for alt_inlines. renderInlines emits ![flatten(alt_inlines)](url). Pinned in AC8.'
status: addressed
---

# Finding

Plan: `{type="image", url, alt="...", title=...}` with alt as a flattened
string.

Goldmark parses alt content as inlines (it can contain emphasis, code spans,
even nested links per CommonMark). Flattening at parse loses that structure —
same kind of loss the rest of the refactor fixes for paragraphs.

In practice, formatted image alt text is rare. But it's inconsistent with the
structure-preservation thesis, and it's a corner that future "fix it" tickets
will hit.

# Resolution

Two options:

1. **Alt as inlines**: `{type="image", url, alt_inlines={...}, title}`.
Renderer flattens via `flattenInlines` to produce `![flat](url)`. Consistent.
2. **Flat alt with explicit caveat**: keep `alt` as a string, drop
inline structure inside alt. Document in the inline-kinds reference: "alt
content is flattened at parse time; nested formatting in alt is not preserved."

(1) is principled, (2) is small. Recommend (1) since we're already doing the
structural work — adding alt as one more `inlines` field is two lines in
`extractInlines` and three in `renderInlines`.

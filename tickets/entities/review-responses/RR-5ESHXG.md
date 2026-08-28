---
id: RR-5ESHXG
type: review-response
title: Every sidebar icon rendered 24x18 — CSS width beat Lucide's presentation attribute
finding: |-
    Lucide emits width/height as PRESENTATION attributes (Icon.js: width: size, height: size). CSS beats presentation attributes, so `.nav-icon { width: 24px }` overrode the 18px width while nothing overrode height=18. Every icon in the sidebar rendered 24x18 — a 4:3 horizontal stretch.

    Measured in real Chrome by the reviewer and independently confirmed: attrW=18, renderedW=24, renderedH=18, in both expanded and collapsed states.

    This is the single thing the ticket set out to fix — visual consistency — shipping with every glyph distorted. Two reasons it survived: a squashed circle still reads as a circle at 18px, and jsdom does not lay out SVG, so no test could observe it. The comment I wrote at that rule asserted width 'keeps the 24px gutter the labels align to' — the intent was right, the mechanism wrong, and the comment made it look considered.

    Also note the diff deleted `text-align: center`, the OLD centering mechanism, without replacing it for the new one.
severity: critical
resolution: |-
    Fixed in bdb197f1. The icon box is now sized by its :size prop and the 24px label gutter comes from margin instead, so sizing and spacing are separate concerns.

    Took two attempts: my first fix used `flex: 0 0 24px`, which on a ROW flex item resolves flex-basis to the main size — i.e. the width — so it reproduced the exact bug. Verified 18x18 in Chrome afterwards rather than assuming.

    Also added Sidebar.iconRender.test.ts, which is the real remedy: it mounts the icon and asserts it is an <svg> with SQUARE width/height and stroke=currentColor. The registry tests only proved resolveIcon returned the right component object; nothing checked that a template rendered it correctly, which is the gap this shipped through.

    A diagnostic note for the next person: chasing this I twice measured a stale build, because `go run` embeds the SPA at compile time and a surviving process kept serving the old asset hash. Comparing the served CSS hash against the on-disk one is the quick way to tell.
status: addressed
---

Found by the cranky-code-reviewer in real Chrome, which is exactly where the
suite could not look. Fixed in `bdb197f1`.

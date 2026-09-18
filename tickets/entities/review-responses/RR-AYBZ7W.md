---
id: RR-AYBZ7W
type: review-response
title: Comment affordances drift when a table with an image is scrolled horizontally
finding: 'BlockCommentOverlay positions affordances as r.top - hostRect.top, which assumes nothing between the block and the host scrolls independently. The new .md-table-scroll wrapper breaks that: an image or diagram in a table cell moves with the table while hostRect does not, so the button detaches from its block. No re-measure fires because ResizeObserver does not observe scroll.'
severity: critical
resolution: observe() now attaches a passive scroll listener calling scan() to every .md-table-scroll inside the host, tracked in a scrollers array and removed in unobserveScrollers() on re-observe and unmount. TextSelectionComment has the same math but is transient and repositions on selectionchange; documented as a known narrower case rather than adding a scroll listener for a rarely-visible popover.
status: addressed
---

# Finding

`BlockCommentOverlay.vue` computes affordance positions as `r.top -
hostRect.top` / `r.left - hostRect.left`, converting a viewport rect to a
host-relative offset. That arithmetic is only valid while **no scrollable box
sits between the block and the host**.

This change introduces exactly such a box. `findCommentableBlocks` uses a
descendant query, so an `<img>` or diagram inside a table cell is still found —
but once the user scrolls that table horizontally, `getBoundingClientRect()`
returns the image's shifted position while `hostRect` does not move. The
affordance detaches from its image and slides across unrelated content.

Worse, nothing re-measures: `observe()` attaches a `ResizeObserver` on the host
and `load` listeners on images. Neither fires on the horizontal scroll of an
inner container, so the button is wrong and **stays** wrong until an unrelated
re-render.

Commentable blocks are `img`, `.mermaid-diagram`, `svg[id^="mermaid"]`,
`.mermaid` and `pre` — all of which can legitimately sit in a table cell.

# Resolution

`observe()` now collects every `.md-table-scroll` inside the host and attaches a
passive `scroll` listener that calls `scan()`. The listeners are tracked in a
`scrollers` array and removed by `unobserveScrollers()` both when re-observing
and in `onBeforeUnmount`, so repeated renders cannot leak them.

Chose re-measuring over suppressing the affordance inside scrollers: a comment
button that works is better than a missing one, and the scroll handler is
passive and only bound when a table wrapper actually exists.

**Not changed, deliberately:** `TextSelectionComment.vue:126-128` uses the same
host-relative math and can drift the same way. It is transient (dismissed on the
next interaction) and repositions on `selectionchange`, and it requires a text
selection made inside a horizontally scrolled cell — a much narrower case. Left
alone rather than binding a scroll listener for a popover that is usually not
mounted. Recorded here so the omission is a decision rather than an oversight.

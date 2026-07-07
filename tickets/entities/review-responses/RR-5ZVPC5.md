---
id: RR-5ZVPC5
type: review-response
title: display:block breaks width:100% fill on narrow tables; comment misrepresents it
finding: 'The shared `.md-table table` rule uses `display: block; overflow-x: auto` to make wide tables scroll without a wrapper. But `display: block` drops table-layout at the outer box, so `width: 100%` no longer fills the container — narrow tables shrink-wrap and hug the left, a visible change vs the old `width: 100%` rules. The docblock claim that tables ''still render normally'' when they fit is inaccurate.'
severity: significant
resolution: 'Moved `overflow-x: auto` from the table onto the `.md-body` container and restored `display: table; width: 100%` on the table itself. Narrow tables now fill the container as before; a table wider than the container scrolls horizontally within the body (the container only grows a scrollbar when a child overflows on x, and tables are the sole wide block). Renamed the class `md-table` -> `md-body` and removed the inaccurate ''renders normally'' comment; the docblock now describes the container-scroll approach accurately.'
status: addressed
---

**Finding:** The shared `.md-table table` rule uses `display: block; overflow-x:
auto` to make wide tables scroll without a wrapper element. But `display: block`
takes the table out of table-layout at the outer box, so `width: 100%` no longer
fills the container — narrow tables shrink-wrap their content and hug the left,
a visible change vs the old `width: 100%` rules on DocumentView/DocumentsPanel.
The docblock's claim that tables "still render normally" when they fit is
inaccurate.

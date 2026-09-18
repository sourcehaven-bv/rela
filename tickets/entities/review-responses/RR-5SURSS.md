---
id: RR-5SURSS
type: review-response
title: display:block on table destroys screen-reader table semantics (WCAG 1.3.1)
finding: The plan's preferred CSS-only mechanism set display:block on .md-body table, which drops the table/row/columnheader roles from the accessibility tree. Engine fixes landed at different times (Safari only 17) and rela declares no browser floor.
severity: critical
resolution: 'Dropped display:block entirely. The table keeps display:table and overflow-x lives on a separate wrapper element, so table semantics and border-collapse are preserved. Verified in-browser: computed display stays ''table''.'
status: addressed
---

# Finding

The plan's preferred CSS-only mechanism set `display: block` on `.md-body
table`. `display` mutates the accessibility tree: `block` on a `<table>` drops
the `table` role and with it `row` / `cell` / `columnheader`. A screen-reader
user loses table navigation and column-header announcement. For the reported
table (a competency rubric where each cell is meaningless without its header)
that is a severe loss.

Engine fixes landed at different times (Firefox 62, Chrome ~80, **Safari only
17**), and rela declares no browser floor, so "modern browsers fixed it" is not
an assumption this project may make silently.

WCAG 2.2 §1.3.1 Info and Relationships (Level A).

# Resolution

Dropped `display: block` entirely. The chosen design keeps the table as
`display: table` and puts `overflow-x` on a separate wrapper element, so table
semantics are preserved. Verified in-browser: computed `display` stays `table`
and `border-collapse` stays `collapse` while the wrapper scrolls.

Also noted: the plan had no accessibility section at all, despite this being a
pure-presentation change whose main risk class is accessibility. An a11y section
is now part of the plan.
